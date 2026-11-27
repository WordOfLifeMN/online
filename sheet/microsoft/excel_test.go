package microsoft

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/WordOfLifeMN/online/sheet"
	"github.com/stretchr/testify/suite"
	"github.com/xuri/excelize/v2"
)

func TestWorkbookTestSuite(t *testing.T) {
	suite.Run(t, new(WorkbookTestSuite))
}

type WorkbookTestSuite struct {
	suite.Suite
	path string
}

var titles = []any{
	"Date", "Name", "Speaker", "Type", "Visibility",
	"Series Name", "Track", "Description", "Thumb", "Video", "Resources",
}

// SetupTest builds a workbook shaped like the one exported from the Google Sheet:
// dates held as serial numbers, whole numbers stored as "1.0", a tab with a Ministry
// column, an ignored "_validate" tab, and a cell whose hyperlink disagrees with its
// text
func (t *WorkbookTestSuite) SetupTest() {
	f := excelize.NewFile()
	defer f.Close()

	iso, err := f.NewStyle(&excelize.Style{CustomNumFmt: strPtr("yyyy-mm-dd")})
	t.Require().NoError(err)
	us, err := f.NewStyle(&excelize.Style{CustomNumFmt: strPtr("m/d/yyyy")})
	t.Require().NoError(err)

	// WOL tab
	t.Require().NoError(f.SetSheetName("Sheet1", "WOL"))
	t.Require().NoError(f.SetSheetRow("WOL", "A1", &titles))

	// row 2: date serial shown as yyyy-mm-dd, as the export wrote it, and a track of "1.0"
	t.Require().NoError(f.SetSheetRow("WOL", "B2", &[]any{"ISO Date", "Pastor Vern", "message", "public", "Grace"}))
	t.Require().NoError(f.SetCellDefault("WOL", "A2", "44254.0"))
	t.Require().NoError(f.SetCellStyle("WOL", "A2", "A2", iso))
	t.Require().NoError(f.SetCellDefault("WOL", "G2", "1.0"))

	// row 3: date serial typed the US way, and a Video of "-" that links to a video
	t.Require().NoError(f.SetSheetRow("WOL", "B3", &[]any{"US Date", "Pastor Vern", "message", "public"}))
	t.Require().NoError(f.SetCellFloat("WOL", "A3", 46353, -1, 64)) // 2026-11-27
	t.Require().NoError(f.SetCellStyle("WOL", "A3", "A3", us))
	t.Require().NoError(f.SetCellStr("WOL", "J3", "-"))
	t.Require().NoError(f.SetCellHyperLink("WOL", "J3", "https://youtu.be/eS6NCaUUc5Q", "External"))

	// row 4: date typed as text
	t.Require().NoError(f.SetSheetRow("WOL", "A4", &[]any{"2026-11-28", "Text Date", "Pastor Vern", "message", "public"}))

	// CORE tab, with its own Ministry column
	_, err = f.NewSheet("CORE")
	t.Require().NoError(err)
	coreTitles := append(append([]any{}, titles[:3]...), append([]any{"Ministry"}, titles[3:]...)...)
	t.Require().NoError(f.SetSheetRow("CORE", "A1", &coreTitles))
	t.Require().NoError(f.SetSheetRow("CORE", "B2", &[]any{"Health", "Speaker", "CORE: Health Matters", "message", "public"}))
	t.Require().NoError(f.SetCellDefault("CORE", "A2", "44254.0"))
	t.Require().NoError(f.SetCellStyle("CORE", "A2", "A2", iso))

	// ignored tab
	_, err = f.NewSheet("_validate")
	t.Require().NoError(err)
	t.Require().NoError(f.SetSheetRow("_validate", "A1", &[]any{"Types"}))

	// when the workbook was saved, as Excel records it
	t.Require().NoError(f.SetDocProps(&excelize.DocProperties{Modified: "2026-11-27T18:05:51Z"}))

	t.path = filepath.Join(t.T().TempDir(), "MediaCatalog.xlsx")
	t.Require().NoError(f.SaveAs(t.path))
}

func strPtr(s string) *string { return &s }

func (t *WorkbookTestSuite) TestOpen() {
	w, err := OpenFile(t.path)
	t.Require().NoError(err)

	t.Equal("MediaCatalog.xlsx", w.Title())
	tabs, err := w.Tabs()
	t.NoError(err)
	t.Equal([]string{"WOL", "CORE", "_validate"}, tabs)

	info, err := os.Stat(t.path)
	t.Require().NoError(err)
	t.Equal(info.ModTime(), w.ModTime())

	// the save time recorded inside the workbook, not the file's time
	saved, ok := w.Saved()
	t.True(ok)
	t.True(saved.Equal(time.Date(2026, 11, 27, 18, 5, 51, 0, time.UTC)), "saved at %s", saved)
	t.Equal(time.Local, saved.Location())
}

func (t *WorkbookTestSuite) TestSaved_NotRecorded() {
	// excelize always records a save time, so copy the fixture without its
	// docProps/core.xml, where the time is kept
	src, err := zip.OpenReader(t.path)
	t.Require().NoError(err)
	defer src.Close()
	var buf bytes.Buffer
	dst := zip.NewWriter(&buf)
	for _, part := range src.File {
		if part.Name == "docProps/core.xml" {
			continue
		}
		t.Require().NoError(dst.Copy(part))
	}
	t.Require().NoError(dst.Close())
	path := filepath.Join(t.T().TempDir(), "bare.xlsx")
	t.Require().NoError(os.WriteFile(path, buf.Bytes(), 0o644))

	w, err := OpenFile(path)
	t.Require().NoError(err)
	_, ok := w.Saved()
	t.False(ok)
}

func (t *WorkbookTestSuite) TestRows() {
	w, err := OpenFile(t.path)
	t.Require().NoError(err)

	rows, err := w.Rows("WOL")
	t.Require().NoError(err)
	t.Require().Len(rows, 4)

	// dates are read by value, whatever their format; text is left as typed
	t.Equal("2021-02-27", rows[1][0])
	t.Equal("2026-11-27", rows[2][0])
	t.Equal("2026-11-28", rows[3][0])

	// a whole number stored as "1.0" reads as displayed
	t.Equal("1", rows[1][6])

	// the cell's text wins over its hyperlink
	t.Equal("-", rows[2][9])
}

func (t *WorkbookTestSuite) TestNewCatalog() {
	w, err := OpenFile(t.path)
	t.Require().NoError(err)

	cat, err := sheet.NewCatalog(w)
	t.Require().NoError(err)
	t.Require().Len(cat.Messages, 4)

	t.Equal("ISO Date", cat.Messages[0].Name)
	t.Equal(catalog.MustParseDateOnly("2021-02-27"), cat.Messages[0].Date)
	t.Equal([]catalog.SeriesReference{{Name: "Grace", Index: 1}}, cat.Messages[0].Series)
	t.Equal(catalog.WordOfLife, cat.Messages[0].Ministry)

	t.Equal(catalog.MustParseDateOnly("2026-11-27"), cat.Messages[1].Date)
	t.Equal("-", cat.Messages[1].Video.URL)

	t.Equal(catalog.MustParseDateOnly("2026-11-28"), cat.Messages[2].Date)

	t.Equal("Health", cat.Messages[3].Name)
	t.Equal(catalog.NewMinistryFromString("CORE: Health Matters"), cat.Messages[3].Ministry)
}

func (t *WorkbookTestSuite) TestReadDoesNotChangeTheWorkbook() {
	before, err := os.ReadFile(t.path)
	t.Require().NoError(err)
	info, err := os.Stat(t.path)
	t.Require().NoError(err)
	modBefore := info.ModTime()

	time.Sleep(20 * time.Millisecond) // so a write would show in the modification time
	w, err := OpenFile(t.path)
	t.Require().NoError(err)
	_, err = sheet.NewCatalog(w)
	t.Require().NoError(err)

	after, err := os.ReadFile(t.path)
	t.Require().NoError(err)
	t.True(bytes.Equal(before, after), "the workbook's contents changed")
	info, err = os.Stat(t.path)
	t.Require().NoError(err)
	t.Equal(modBefore, info.ModTime())
}

func (t *WorkbookTestSuite) TestOpen_Errors() {
	dir := t.T().TempDir()

	_, err := OpenFile(filepath.Join(dir, "missing.xlsx"))
	t.ErrorContains(err, "missing.xlsx")

	csv := filepath.Join(dir, "catalog.csv")
	t.Require().NoError(os.WriteFile(csv, []byte("a,b\n"), 0o644))
	_, err = OpenFile(csv)
	t.ErrorContains(err, "only .xlsx and .xlsm")

	junk := filepath.Join(dir, "junk.xlsx")
	t.Require().NoError(os.WriteFile(junk, []byte("not a zip"), 0o644))
	_, err = OpenFile(junk)
	t.ErrorContains(err, "junk.xlsx")
}
