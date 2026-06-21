package excelclient

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/WordOfLifeMN/online/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

const testCatalogPath = "../testdata/excel-test-catalog.xlsx"

func TestMain(m *testing.M) {
	if err := createTestCatalog(testCatalogPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test fixtures: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// createTestCatalog builds a fixture workbook at path with:
//   - "Series"       tab: 2 series rows
//   - "Messages"     tab: 2 message rows
//   - "Messages-2019" tab: 1 message row (tests multi-tab merging)
//   - "_Metadata"    tab: should be skipped by NewCatalog
func createTestCatalog(path string) error {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", "Series")

	if err := writeSeriesSheet(f, "Series"); err != nil {
		return err
	}
	if err := writeMessagesSheet(f, "Messages"); err != nil {
		return err
	}
	if err := writeOldMessagesSheet(f, "Messages-2019"); err != nil {
		return err
	}

	f.NewSheet("_Metadata")
	f.SetCellValue("_Metadata", "A1", "this sheet should be ignored")

	return f.SaveAs(path)
}

func writeSeriesSheet(f *excelize.File, sheet string) error {
	headers := []string{
		"ID", "Name", "Description",
		"Date Started", "Date Ended",
		"Booklets", "Visibility",
		"CD Jacket", "DVD Jacket", "Cover Art",
	}
	if err := writeRow(f, sheet, 1, toAnySlice(headers)); err != nil {
		return err
	}

	rows := [][]any{
		{
			"seri-001",
			"Alpha Series",
			"First test series",
			time.Date(2020, 1, 5, 0, 0, 0, 0, time.UTC),
			time.Date(2020, 6, 28, 0, 0, 0, 0, time.UTC),
			"",
			"public",
			"",
			"https://example.com/dvd.jpg",
			"https://example.com/thumb.jpg",
		},
		{
			"seri-002",
			"Beta Series",
			"Second test series",
			time.Date(2021, 3, 1, 0, 0, 0, 0, time.UTC),
			"",
			"",
			"private",
			"https://example.com/cd.jpg",
			"",
			"",
		},
	}
	for i, row := range rows {
		if err := writeRow(f, sheet, i+2, row); err != nil {
			return err
		}
	}
	return nil
}

func writeMessagesSheet(f *excelize.File, sheet string) error {
	f.NewSheet(sheet)
	headers := messageHeaders()
	if err := writeRow(f, sheet, 1, toAnySlice(headers)); err != nil {
		return err
	}

	rows := [][]any{
		{
			"First Message",
			time.Date(2020, 1, 5, 0, 0, 0, 0, time.UTC),
			"John Doe;Jane Smith",
			"wol",
			"message",
			"public",
			"Alpha Series",
			"1",
			"First test message",
			"https://example.com/audio1.mp3",
			"",
			"",
		},
		{
			"Second Message",
			time.Date(2020, 1, 12, 0, 0, 0, 0, time.UTC),
			"John Doe",
			"wol",
			"message",
			"public",
			"Alpha Series",
			"2",
			"Second test message",
			"",
			"https://example.com/video2.mp4",
			"",
		},
	}
	for i, row := range rows {
		if err := writeRow(f, sheet, i+2, row); err != nil {
			return err
		}
	}
	return nil
}

func writeOldMessagesSheet(f *excelize.File, sheet string) error {
	f.NewSheet(sheet)
	headers := messageHeaders()
	if err := writeRow(f, sheet, 1, toAnySlice(headers)); err != nil {
		return err
	}
	row := []any{
		"Old Message",
		time.Date(2019, 5, 19, 0, 0, 0, 0, time.UTC),
		"Jane Smith",
		"tbo",
		"testimony",
		"private",
		"",
		"",
		"An old message",
		"",
		"",
		"",
	}
	return writeRow(f, sheet, 2, row)
}

func messageHeaders() []string {
	return []string{
		"Name", "Date", "Speaker", "Ministry", "Type", "Visibility",
		"Series Name", "Track", "Description", "Audio Link", "Video Link", "Resources",
	}
}

func writeRow(f *excelize.File, sheet string, rowNum int, values []any) error {
	for col, val := range values {
		cell, err := excelize.CoordinatesToCellName(col+1, rowNum)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, val); err != nil {
			return err
		}
	}
	return nil
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewCatalog_InvalidPath(t *testing.T) {
	cat, err := NewCatalog("../testdata/does-not-exist.xlsx")
	require.Error(t, err)
	require.NotNil(t, cat)
}

func TestNewCatalog_ReadsSeries(t *testing.T) {
	cat, err := NewCatalog(testCatalogPath)
	require.NoError(t, err)
	require.Len(t, cat.Series, 2)

	s1 := cat.Series[0]
	assert.Equal(t, "seri-001", s1.ID)
	assert.Equal(t, "Alpha Series", s1.Name)
	assert.Equal(t, "First test series", s1.Description)
	assert.Equal(t, "2020-01-05", s1.StartDate.String())
	assert.Equal(t, "2020-06-28", s1.StopDate.String())
	assert.Equal(t, catalog.Public, s1.Visibility)
	assert.Equal(t, "https://example.com/dvd.jpg", s1.Jacket)
	assert.Equal(t, "https://example.com/thumb.jpg", s1.Thumbnail)

	s2 := cat.Series[1]
	assert.Equal(t, "seri-002", s2.ID)
	assert.Equal(t, catalog.Private, s2.Visibility)
	assert.Equal(t, "https://example.com/cd.jpg", s2.Jacket, "should fall back to CD jacket when DVD jacket is absent")
	assert.True(t, s2.StopDate.IsZero(), "end date should be zero when cell is empty")
}

func TestNewCatalog_ReadsMessages(t *testing.T) {
	cat, err := NewCatalog(testCatalogPath)
	require.NoError(t, err)

	// 2 in Messages + 1 in Messages-2019
	require.Len(t, cat.Messages, 3)

	msg := cat.Messages[0]
	assert.Equal(t, "First Message", msg.Name)
	assert.Equal(t, "2020-01-05", msg.Date.String())
	assert.Equal(t, []string{"John Doe", "Jane Smith"}, msg.Speakers)
	assert.Equal(t, catalog.WordOfLife, msg.Ministry)
	assert.Equal(t, catalog.Message, msg.Type)
	assert.Equal(t, catalog.Public, msg.Visibility)
	assert.Equal(t, "https://example.com/audio1.mp3", msg.Audio.URL)
	assert.Empty(t, msg.Video.URL)
	require.Len(t, msg.Series, 1)
	assert.Equal(t, "Alpha Series", msg.Series[0].Name)
	assert.Equal(t, 1, msg.Series[0].Index)

	msg2 := cat.Messages[1]
	assert.Equal(t, "2020-01-12", msg2.Date.String())
	assert.Equal(t, "https://example.com/video2.mp4", msg2.Video.URL)
	assert.Empty(t, msg2.Audio.URL)
	assert.Equal(t, 2, msg2.Series[0].Index)
}

func TestNewCatalog_SkipsUnderscoreTabs(t *testing.T) {
	cat, err := NewCatalog(testCatalogPath)
	require.NoError(t, err)
	// _Metadata tab must not contribute any rows
	assert.Len(t, cat.Series, 2)
	assert.Len(t, cat.Messages, 3)
}

func TestNewCatalog_ReadsMessagesFromMultipleTabs(t *testing.T) {
	cat, err := NewCatalog(testCatalogPath)
	require.NoError(t, err)

	var oldMsg *catalog.CatalogMessage
	for i := range cat.Messages {
		if cat.Messages[i].Name == "Old Message" {
			oldMsg = &cat.Messages[i]
			break
		}
	}
	require.NotNil(t, oldMsg, "message from Messages-2019 tab not found")
	assert.Equal(t, "2019-05-19", oldMsg.Date.String())
	assert.Equal(t, catalog.TheBridgeOutreach, oldMsg.Ministry)
	assert.Equal(t, catalog.Testimony, oldMsg.Type)
	assert.Equal(t, catalog.Private, oldMsg.Visibility)
}
