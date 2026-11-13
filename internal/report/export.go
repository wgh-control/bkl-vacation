// Package report renders the HR download as CSV, XLSX, legacy XML XLS, and PDF.
package report

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"

	_ "embed"

	"github.com/go-pdf/fpdf"
)

//go:embed font/DejaVuSans.ttf
var dejavuTTF []byte

// Table is a header row plus body rows.
type Table struct {
	Headers []string
	Rows    [][]string
}

func CSV(t Table) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write(t.Headers)
	for _, row := range t.Rows {
		_ = w.Write(row)
	}
	w.Flush()
	return buf.Bytes()
}

// XLS is SpreadsheetML that Excel opens as a workbook. The HR download calls this XLS.
func XLS(t Table) []byte {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<?mso-application progid="Excel.Sheet"?>` + "\n")
	buf.WriteString(`<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">`)
	buf.WriteString(`<Worksheet ss:Name="leave"><Table>`)
	writeXLSRow(&buf, t.Headers)
	for _, row := range t.Rows {
		writeXLSRow(&buf, row)
	}
	buf.WriteString(`</Table></Worksheet></Workbook>`)
	return buf.Bytes()
}

func writeXLSRow(buf *bytes.Buffer, cells []string) {
	buf.WriteString(`<Row>`)
	for _, c := range cells {
		buf.WriteString(`<Cell><Data ss:Type="String">`)
		_ = xml.EscapeText(buf, []byte(c))
		buf.WriteString(`</Data></Cell>`)
	}
	buf.WriteString(`</Row>`)
}

// XLSX is a minimal Office Open XML workbook.
func XLSX(t Table) ([]byte, error) {
	var shared []string
	index := map[string]int{}
	cell := func(v string) int {
		if n, ok := index[v]; ok {
			return n
		}
		n := len(shared)
		index[v] = n
		shared = append(shared, v)
		return n
	}
	type ref struct{ row []int }
	var grid []ref
	grid = append(grid, ref{row: mapRow(t.Headers, cell)})
	for _, row := range t.Rows {
		grid = append(grid, ref{row: mapRow(row, cell)})
	}
	var ss bytes.Buffer
	ss.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	ss.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="` + fmt.Sprint(len(shared)) + `" uniqueCount="` + fmt.Sprint(len(shared)) + `">`)
	for _, s := range shared {
		ss.WriteString(`<si><t xml:space="preserve">`)
		_ = xml.EscapeText(&ss, []byte(s))
		ss.WriteString(`</t></si>`)
	}
	ss.WriteString(`</sst>`)

	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r, row := range grid {
		sheet.WriteString(fmt.Sprintf(`<row r="%d">`, r+1))
		for c, idx := range row.row {
			sheet.WriteString(fmt.Sprintf(`<c r="%s%d" t="s"><v>%d</v></c>`, colName(c), r+1, idx))
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="leave" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": sheet.String(),
		"xl/sharedStrings.xml":     ss.String(),
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mapRow(cells []string, cell func(string) int) []int {
	out := make([]int, len(cells))
	for i, c := range cells {
		out[i] = cell(c)
	}
	return out
}

func colName(i int) string {
	n := i + 1
	var s string
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

// PDF renders a landscape table. DejaVu Sans covers Russian and English.
func PDF(t Table) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.AddUTF8FontFromBytes("dejavu", "", dejavuTTF)
	pdf.SetFont("dejavu", "", 8)
	pdf.SetMargins(8, 8, 8)
	pdf.AddPage()
	pdf.CellFormat(0, 8, "BKL", "", 1, "L", false, 0, "")
	widths := fitWidths(len(t.Headers), 281)
	writePDFRow(pdf, t.Headers, widths, true)
	for _, row := range t.Rows {
		writePDFRow(pdf, row, widths, false)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	if pdf.Err() {
		return nil, pdf.Error()
	}
	return buf.Bytes(), nil
}

func writePDFRow(pdf *fpdf.Fpdf, cells []string, widths []float64, header bool) {
	if header {
		pdf.SetFillColor(230, 230, 230)
	}
	_, pageH := pdf.GetPageSize()
	if pdf.GetY() > pageH-16 {
		pdf.AddPage()
	}
	startX := pdf.GetX()
	y := pdf.GetY()
	height := 6.0
	for i, c := range cells {
		w := 20.0
		if i < len(widths) {
			w = widths[i]
		}
		pdf.SetXY(startX, y)
		pdf.CellFormat(w, height, clip(c, 40), "1", 0, "L", header, 0, "")
		startX += w
	}
	pdf.SetXY(8, y+height)
}

func fitWidths(n int, total float64) []float64 {
	if n == 0 {
		return nil
	}
	w := total / float64(n)
	out := make([]float64, n)
	for i := range out {
		out[i] = w
	}
	return out
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
