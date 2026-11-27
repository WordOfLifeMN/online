//go:build windows

package microsoft

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

// CRITICAL: only property READS go through COM - Workbooks, Name, FullName, Date1904,
// Worksheets, UsedRange, Row, Column, Value2. No setter, no method that saves,
// activates, selects or recalculates. Excel holds the operator's live session and the
// system of record; see the read-only comment in excel.go.

var (
	// liveBusyFor is how long a busy Excel is retried before the file is read instead,
	// and liveBusyEvery how often. Excel rejects calls at once while the operator is
	// typing in a cell, so this is a quick poll, not a wait on a hung call.
	liveBusyFor   = 2 * time.Second
	liveBusyEvery = 150 * time.Millisecond

	// liveTimeout bounds the whole attempt, in case a call hangs instead of being
	// rejected
	liveTimeout = 10 * time.Second
)

const (
	rpcCallRejected  = 0x80010001 // RPC_E_CALL_REJECTED: Excel is busy, e.g. mid-edit
	rpcRetryLater    = 0x8001010A // RPC_E_SERVERCALL_RETRYLATER
	coInitAlreadyDid = 1          // S_FALSE: COM was already initialized on this thread
)

var (
	oleaut32                  = windows.NewLazySystemDLL("oleaut32.dll")
	procSafeArrayAccessData   = oleaut32.NewProc("SafeArrayAccessData")
	procSafeArrayUnaccessData = oleaut32.NewProc("SafeArrayUnaccessData")
	procSafeArrayGetDim       = oleaut32.NewProc("SafeArrayGetDim")
)

// readLive reads the workbook at path from the running Excel. COM needs a thread of
// its own, so the work is done on one, and abandoned if it doesn't finish in time.
func readLive(path string) (*liveWorkbook, error) {
	type result struct {
		workbook *liveWorkbook
		err      error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		wb, err := readLiveOnThread(path)
		done <- result{wb, err}
	}()

	select {
	case r := <-done:
		return r.workbook, r.err
	case <-time.After(liveTimeout):
		return nil, errLiveTimeout
	}
}

// readLiveOnThread does the reading on a thread locked for COM, retrying while Excel
// is busy
func readLiveOnThread(path string) (*liveWorkbook, error) {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil && !hresultIs(err, coInitAlreadyDid) {
		return nil, fmt.Errorf("cannot start COM: %w", err)
	}
	defer ole.CoUninitialize()

	deadline := time.Now().Add(liveBusyFor)
	for {
		wb, err := readLiveOnce(path)
		if err == nil || !isBusy(err) {
			return wb, err
		}
		if time.Now().After(deadline) {
			return nil, errBusy
		}
		time.Sleep(liveBusyEvery)
	}
}

// readLiveOnce makes one attempt to find the workbook in Excel and read every sheet
func readLiveOnce(path string) (*liveWorkbook, error) {
	unknown, err := oleutil.GetActiveObject("Excel.Application")
	if err != nil {
		switch {
		case isBusy(err):
			return nil, err
		case excelProcessRunning():
			return nil, errUnreachable
		default:
			return nil, errNotRunning
		}
	}
	defer unknown.Release()
	excel, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, err
	}
	defer excel.Release()

	// every property read is kept, and cleared - which releases what it holds - when
	// this attempt is over
	var held []*ole.VARIANT
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].Clear()
		}
	}()
	get := func(d *ole.IDispatch, name string, args ...any) (*ole.VARIANT, error) {
		v, err := oleutil.GetProperty(d, name, args...)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		held = append(held, v)
		return v, nil
	}

	// find the workbook
	workbooks, err := get(excel, "Workbooks")
	if err != nil {
		return nil, err
	}
	count, err := get(workbooks.ToIDispatch(), "Count")
	if err != nil {
		return nil, err
	}
	var found *ole.IDispatch
	var name string
	matches := 0
	for i := 1; i <= toInt(count.Value()); i++ {
		w, err := get(workbooks.ToIDispatch(), "Item", i)
		if err != nil {
			return nil, err
		}
		n, err := get(w.ToIDispatch(), "Name")
		if err != nil {
			return nil, err
		}
		full, err := get(w.ToIDispatch(), "FullName")
		if err != nil {
			return nil, err
		}
		if matchWorkbook(path, n.ToString(), full.ToString()) {
			matches++
			found, name = w.ToIDispatch(), n.ToString()
		}
	}
	switch {
	case matches == 0:
		return nil, errNotOpen
	case matches > 1:
		return nil, errAmbiguous
	}

	date1904 := false
	if v, err := get(found, "Date1904"); err == nil {
		date1904, _ = v.Value().(bool)
	}

	// read every worksheet, one call per sheet for its values
	sheets, err := get(found, "Worksheets")
	if err != nil {
		return nil, err
	}
	sheetCount, err := get(sheets.ToIDispatch(), "Count")
	if err != nil {
		return nil, err
	}
	wb := &liveWorkbook{name: name, rows: map[string][][]string{}}
	for i := 1; i <= toInt(sheetCount.Value()); i++ {
		sheet, err := get(sheets.ToIDispatch(), "Item", i)
		if err != nil {
			return nil, err
		}
		tab, err := get(sheet.ToIDispatch(), "Name")
		if err != nil {
			return nil, err
		}
		used, err := get(sheet.ToIDispatch(), "UsedRange")
		if err != nil {
			return nil, err
		}
		firstRow, err := get(used.ToIDispatch(), "Row")
		if err != nil {
			return nil, err
		}
		firstCol, err := get(used.ToIDispatch(), "Column")
		if err != nil {
			return nil, err
		}
		values, err := get(used.ToIDispatch(), "Value2")
		if err != nil {
			return nil, err
		}
		grid, err := variantGrid(values)
		if err != nil {
			return nil, fmt.Errorf("sheet %s: %w", tab.ToString(), err)
		}
		wb.tabs = append(wb.tabs, tab.ToString())
		wb.rows[tab.ToString()] = gridToRows(grid, toInt(firstRow.Value()), toInt(firstCol.Value()), date1904)
	}
	return wb, nil
}

// variantGrid copies a range's values out of the VARIANT COM returned. A multi-cell
// range is a 2-D SAFEARRAY of VARIANTs, stored column by column. A one-cell range is
// a single value.
//
// The array is read through SafeArrayAccessData: go-ole's ToValueArray walks only the
// first dimension, and its GetDimensions returns the count cast to a pointer, which
// crashes the garbage collector, so neither is used.
func variantGrid(v *ole.VARIANT) ([][]any, error) {
	if v.VT&ole.VT_ARRAY == 0 {
		return [][]any{{variantValue(v)}}, nil
	}

	array := v.ToArray()
	if dims, _, _ := procSafeArrayGetDim.Call(uintptr(unsafe.Pointer(array.Array))); dims != 2 {
		return nil, fmt.Errorf("expected a 2-D range, got %d dimensions", dims)
	}
	rows, err := array.TotalElements(1)
	if err != nil {
		return nil, err
	}
	cols, err := array.TotalElements(2)
	if err != nil {
		return nil, err
	}

	var data unsafe.Pointer
	if hr, _, _ := procSafeArrayAccessData.Call(uintptr(unsafe.Pointer(array.Array)), uintptr(unsafe.Pointer(&data))); hr != 0 {
		return nil, ole.NewError(hr)
	}
	defer procSafeArrayUnaccessData.Call(uintptr(unsafe.Pointer(array.Array)))

	cells := unsafe.Slice((*ole.VARIANT)(data), int(rows)*int(cols))
	flat := make([]any, len(cells))
	for i := range cells {
		flat[i] = variantValue(&cells[i])
	}
	return fromColumnMajor(flat, int(rows), int(cols)), nil
}

// variantValue is one cell's value: nil, string, float64, bool or cellError. Value2
// gives dates as plain numbers.
func variantValue(v *ole.VARIANT) any {
	switch v.VT {
	case ole.VT_EMPTY, ole.VT_NULL:
		return nil
	case ole.VT_ERROR:
		return cellError(uint32(v.Val) & 0xFFFF)
	case ole.VT_BOOL:
		return v.Val != 0
	default:
		return v.Value()
	}
}

// toInt reads a COM integer, whichever size it came as
func toInt(v any) int {
	switch x := v.(type) {
	case int32:
		return int(x)
	case int64:
		return int(x)
	case int:
		return x
	case int16:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

// isBusy reports whether Excel turned a call away because it is busy
func isBusy(err error) bool {
	return hresultIs(err, rpcCallRejected) || hresultIs(err, rpcRetryLater)
}

// hresultIs reports whether err is a COM error with the given code
func hresultIs(err error, code uintptr) bool {
	var oe *ole.OleError
	return errors.As(err, &oe) && oe.Code() == code
}

// excelProcessRunning reports whether an Excel process exists at all, to tell "Excel
// isn't running" from "Excel is running but COM can't see it", which is what happens
// when one of the two runs as Administrator
func excelProcessRunning() bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if strings.EqualFold(filepath.Base(windows.UTF16ToString(entry.ExeFile[:])), "EXCEL.EXE") {
			return true
		}
	}
	return false
}
