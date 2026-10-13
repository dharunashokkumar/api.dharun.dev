package dataset

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// readXLSX reads every sheet of a workbook as rows of text, by sheet name. It knows just
// enough of the format for plain tables: shared strings, inline strings and numbers.
// Formatting, formulas and dates are not interpreted.
func readXLSX(raw []byte) (map[string][][]string, error) {
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("not a workbook: %w", err)
	}
	open := func(name string) ([]byte, error) {
		for _, file := range archive.File {
			if file.Name == name {
				reader, err := file.Open()
				if err != nil {
					return nil, err
				}
				defer reader.Close()
				// A worksheet is plain XML; 100 MB is far beyond any table this reads.
				return io.ReadAll(io.LimitReader(reader, 100<<20))
			}
		}
		return nil, fmt.Errorf("%s missing from the workbook", name)
	}

	var shared []string
	if data, err := open("xl/sharedStrings.xml"); err == nil {
		var table struct {
			Items []struct {
				Text string `xml:"t"`
				Runs []struct {
					Text string `xml:"t"`
				} `xml:"r"`
			} `xml:"si"`
		}
		if err := xml.Unmarshal(data, &table); err != nil {
			return nil, fmt.Errorf("shared strings: %w", err)
		}
		for _, item := range table.Items {
			text := item.Text
			for _, run := range item.Runs {
				text += run.Text
			}
			shared = append(shared, text)
		}
	}

	workbook, err := open("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var book struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(workbook, &book); err != nil {
		return nil, fmt.Errorf("workbook: %w", err)
	}
	relations, err := open("xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(relations, &rels); err != nil {
		return nil, fmt.Errorf("relationships: %w", err)
	}
	targets := map[string]string{}
	for _, item := range rels.Items {
		targets[item.ID] = path.Join("xl", strings.TrimPrefix(item.Target, "/xl/"))
	}

	sheets := map[string][][]string{}
	for _, sheet := range book.Sheets {
		data, err := open(targets[sheet.ID])
		if err != nil {
			return nil, err
		}
		var content struct {
			Rows []struct {
				Cells []struct {
					Ref    string `xml:"r,attr"`
					Type   string `xml:"t,attr"`
					Value  string `xml:"v"`
					Inline string `xml:"is>t"`
				} `xml:"c"`
			} `xml:"sheetData>row"`
		}
		if err := xml.Unmarshal(data, &content); err != nil {
			return nil, fmt.Errorf("sheet %s: %w", sheet.Name, err)
		}
		var rows [][]string
		for _, row := range content.Rows {
			var cells []string
			for _, cell := range row.Cells {
				column := columnIndex(cell.Ref)
				for len(cells) <= column {
					cells = append(cells, "")
				}
				switch cell.Type {
				case "s":
					index, err := strconv.Atoi(cell.Value)
					if err != nil || index < 0 || index >= len(shared) {
						return nil, fmt.Errorf("sheet %s cell %s: bad shared string %q", sheet.Name, cell.Ref, cell.Value)
					}
					cells[column] = shared[index]
				case "inlineStr":
					cells[column] = cell.Inline
				default:
					cells[column] = cell.Value
				}
			}
			rows = append(rows, cells)
		}
		sheets[sheet.Name] = rows
	}
	return sheets, nil
}

// columnIndex turns the letters of a cell reference ("AB12") into a column number.
func columnIndex(ref string) int {
	index := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			break
		}
		index = index*26 + int(r-'A'+1)
	}
	return index - 1
}
