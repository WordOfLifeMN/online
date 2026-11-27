package microsoft

// turning cell values into the text a sheet.Source returns, shared by the file reader
// and the running-Excel reader so a value can't read one way from one and another way
// from the other

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/WordOfLifeMN/online/sheet"
	"github.com/xuri/excelize/v2"
)

// serialDate turns an Excel date serial into yyyy-mm-dd. False for a number that can't
// be a date.
func serialDate(serial float64, date1904 bool) (string, bool) {
	if serial < 1 {
		return "", false
	}
	t, err := excelize.ExcelDateToTime(serial, date1904)
	if err != nil {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

// cellError is an Excel error value, such as #N/A, by its code: 2042 for #N/A
type cellError uint32

// String is the error as Excel shows it
func (e cellError) String() string {
	switch e {
	case 2000:
		return "#NULL!"
	case 2007:
		return "#DIV/0!"
	case 2015:
		return "#VALUE!"
	case 2023:
		return "#REF!"
	case 2029:
		return "#NAME?"
	case 2036:
		return "#NUM!"
	case 2042:
		return "#N/A"
	case 2043:
		return "#GETTING_DATA"
	default:
		return "#ERROR"
	}
}

// cellText is a raw cell value as text, the way the file reader would show it for the
// catalog's columns. A whole number reads "1", not "1.0".
func cellText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return numberText(x)
	case float32:
		return numberText(float64(x))
	case bool:
		if x {
			return "TRUE"
		}
		return "FALSE"
	case cellError:
		return x.String()
	case time.Time:
		return x.Format("2006-01-02")
	default:
		return fmt.Sprint(x)
	}
}

// numberText writes a number the way Excel's General format would, closely enough for
// the catalog: whole numbers without a decimal point, others to 15 significant digits
// so 0.1+0.2 reads "0.3"
func numberText(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(f, 'g', 15, 64), 64)
	if err != nil {
		rounded = f
	}
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

// fromColumnMajor rearranges a range's values, stored column by column as COM returns
// them, into rows
func fromColumnMajor(flat []any, rows, cols int) [][]any {
	grid := make([][]any, rows)
	for r := range grid {
		grid[r] = make([]any, cols)
		for c := range grid[r] {
			if i := r + c*rows; i < len(flat) {
				grid[r][c] = flat[i]
			}
		}
	}
	return grid
}

// gridToRows turns a range of raw values into the rows a sheet.Source returns.
//
// firstRow and firstCol are where the range starts on the sheet, 1-based, so the rows
// are padded to put the sheet's row 1 first and column A first. The Date column, found
// by its title in row 1, is read by value, as the file reader reads it. Trailing empty
// cells and rows are dropped.
func gridToRows(grid [][]any, firstRow, firstCol int, date1904 bool) [][]string {
	padRows, padCols := max(firstRow-1, 0), max(firstCol-1, 0)

	rows := make([][]string, padRows, padRows+len(grid))
	for i := range rows {
		rows[i] = []string{}
	}
	for _, values := range grid {
		row := make([]string, padCols+len(values))
		for c, v := range values {
			row[padCols+c] = cellText(v)
		}
		rows = append(rows, row)
	}

	// the Date column holds date serials: read them as dates
	if len(rows) > 0 {
		for col, title := range rows[0] {
			if title != sheet.DateColumn || col < padCols {
				continue
			}
			for r := max(1, padRows); r < len(rows); r++ {
				values := grid[r-padRows]
				if serial, ok := values[col-padCols].(float64); ok {
					if date, ok := serialDate(serial, date1904); ok {
						rows[r][col] = date
					}
				}
			}
		}
	}

	// drop trailing empty cells, then trailing empty rows
	for r, row := range rows {
		end := len(row)
		for end > 0 && strings.TrimSpace(row[end-1]) == "" {
			end--
		}
		rows[r] = row[:end]
	}
	end := len(rows)
	for end > 0 && len(rows[end-1]) == 0 {
		end--
	}
	return rows[:end]
}
