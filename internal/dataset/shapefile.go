package dataset

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
)

// A minimal reader for ESRI shapefiles of polygons and their dBASE attribute tables:
// just what the constituency maps need.

// ring is a closed outline as alternating longitude, latitude pairs.
type ring []float64

// readPolygons reads the polygons of a .shp file, one entry per record (nil for a null
// shape). Each polygon is its rings; holes are told apart by the even-odd rule.
func readPolygons(shp []byte) ([][]ring, error) {
	if len(shp) < 100 || binary.BigEndian.Uint32(shp[0:4]) != 9994 {
		return nil, errors.New("not a shapefile")
	}
	var shapes [][]ring
	for offset := 100; offset+8 <= len(shp); {
		length := int(binary.BigEndian.Uint32(shp[offset+4:offset+8])) * 2
		content := offset + 8
		if content+length > len(shp) {
			return nil, fmt.Errorf("record at byte %d runs past the end", offset)
		}
		record := shp[content : content+length]
		offset = content + length
		kind := binary.LittleEndian.Uint32(record[0:4])
		if kind == 0 {
			shapes = append(shapes, nil)
			continue
		}
		if kind != 5 || len(record) < 44 {
			return nil, fmt.Errorf("shape type %d is not a polygon", kind)
		}
		parts := int(binary.LittleEndian.Uint32(record[36:40]))
		points := int(binary.LittleEndian.Uint32(record[40:44]))
		if 44+4*parts+16*points > len(record) {
			return nil, errors.New("polygon record is shorter than its counts")
		}
		starts := make([]int, parts+1)
		for i := 0; i < parts; i++ {
			starts[i] = int(binary.LittleEndian.Uint32(record[44+4*i:]))
		}
		starts[parts] = points
		base := 44 + 4*parts
		var polygon []ring
		for i := 0; i < parts; i++ {
			var r ring
			for p := starts[i]; p < starts[i+1]; p++ {
				x := math.Float64frombits(binary.LittleEndian.Uint64(record[base+16*p:]))
				y := math.Float64frombits(binary.LittleEndian.Uint64(record[base+16*p+8:]))
				r = append(r, x, y)
			}
			polygon = append(polygon, r)
		}
		shapes = append(shapes, polygon)
	}
	return shapes, nil
}

// readDBF reads a dBASE table into rows of named, trimmed text fields.
func readDBF(dbf []byte) ([]map[string]string, error) {
	if len(dbf) < 32 {
		return nil, errors.New("not a dBASE file")
	}
	count := int(binary.LittleEndian.Uint32(dbf[4:8]))
	headerLength := int(binary.LittleEndian.Uint16(dbf[8:10]))
	recordLength := int(binary.LittleEndian.Uint16(dbf[10:12]))
	type field struct {
		name   string
		length int
	}
	var fields []field
	for offset := 32; offset+32 <= headerLength && dbf[offset] != 0x0D; offset += 32 {
		fields = append(fields, field{strings.TrimRight(string(dbf[offset:offset+11]), "\x00"), int(dbf[offset+16])})
	}
	if headerLength+count*recordLength > len(dbf) {
		return nil, errors.New("dBASE file is shorter than its record count")
	}
	rows := make([]map[string]string, count)
	for i := 0; i < count; i++ {
		record := dbf[headerLength+i*recordLength : headerLength+(i+1)*recordLength]
		row := map[string]string{}
		position := 1 // past the deletion flag
		for _, f := range fields {
			// The files are Latin-1; the names this reads are plain ASCII.
			row[f.name] = strings.TrimSpace(string(record[position : position+f.length]))
			position += f.length
		}
		rows[i] = row
	}
	return rows, nil
}

// encodeRings packs rings as little-endian float32 pairs, about a metre of precision:
// count of rings, then for each a count of points and the points.
func encodeRings(rings []ring) []byte {
	size := 4
	for _, r := range rings {
		size += 4 + 4*len(r)
	}
	out := make([]byte, 0, size)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(rings)))
	for _, r := range rings {
		out = binary.LittleEndian.AppendUint32(out, uint32(len(r)/2))
		for _, value := range r {
			out = binary.LittleEndian.AppendUint32(out, math.Float32bits(float32(value)))
		}
	}
	return out
}

// DecodeRings unpacks what encodeRings packed.
func DecodeRings(raw []byte) ([][]float32, error) {
	if len(raw) < 4 {
		return nil, errors.New("no rings")
	}
	count := int(binary.LittleEndian.Uint32(raw))
	offset := 4
	rings := make([][]float32, 0, count)
	for i := 0; i < count; i++ {
		if offset+4 > len(raw) {
			return nil, errors.New("rings cut short")
		}
		points := int(binary.LittleEndian.Uint32(raw[offset:]))
		offset += 4
		if offset+8*points > len(raw) {
			return nil, errors.New("ring cut short")
		}
		r := make([]float32, 2*points)
		for j := range r {
			r[j] = math.Float32frombits(binary.LittleEndian.Uint32(raw[offset+4*j:]))
		}
		offset += 8 * points
		rings = append(rings, r)
	}
	return rings, nil
}

// Contains reports whether a point lies inside rings, by the even-odd rule, so holes
// and multi-part shapes need no special handling.
func Contains(rings [][]float32, lng, lat float64) bool {
	inside := false
	for _, r := range rings {
		n := len(r) / 2
		for i, j := 0, n-1; i < n; j, i = i, i+1 {
			xi, yi := float64(r[2*i]), float64(r[2*i+1])
			xj, yj := float64(r[2*j]), float64(r[2*j+1])
			if (yi > lat) != (yj > lat) && lng < (xj-xi)*(lat-yi)/(yj-yi)+xi {
				inside = !inside
			}
		}
	}
	return inside
}

func bounds(rings []ring) (minLng, maxLng, minLat, maxLat float64) {
	minLng, minLat = math.Inf(1), math.Inf(1)
	maxLng, maxLat = math.Inf(-1), math.Inf(-1)
	for _, r := range rings {
		for i := 0; i+1 < len(r); i += 2 {
			minLng, maxLng = math.Min(minLng, r[i]), math.Max(maxLng, r[i])
			minLat, maxLat = math.Min(minLat, r[i+1]), math.Max(maxLat, r[i+1])
		}
	}
	return
}
