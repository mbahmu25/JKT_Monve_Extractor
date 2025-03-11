package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	// geojson "github.com/paulmach/go.geojson"
	"encoding/csv"
	"encoding/json"
)

type Point struct {
	X float64
	Y float64
	Z float64
}
type Extent struct {
	maxX float64
	maxY float64
	minX float64
	minY float64
}
type MultiPolygon struct {
	outer  []Point
	hole   []Point
	island []*MultiPolygon
}
type Faces struct {
	v  int
	vt int
	vn int
}

type Tiles struct {
	extent     Extent
	childTiles []*Tiles
	index      []int
}

func main() {
	filePath := os.Args
	filePathGeojson := os.Args
	fmt.Println(filePath, filePathGeojson)
	var geojson map[string]interface{}
	data := ReadFile(filePath[1])
	geoJSONString := ReadFile(filePathGeojson[2])
	err := json.Unmarshal(geoJSONString, &geojson)
	if err != nil {
		fmt.Println(err)
	}

	var v, vn, Mesh = ReadMesh(data)
	geoPolygon, extent := ReadGeomGeojson(geojson)
	cent := []Point{}
	index := []int{}

	fmt.Println("Number of Object to extract: ", len(Mesh))
	// Proses Tiling agar mengurangi search pada geojson
	tiles := CreateTiles(extent, 500, geoPolygon)
	for i := 0; i < len(Mesh); i++ {
		// cent = append(cent, Point{cx, cy, 0})
		index = append(index, SearchIdInGeom(Mesh, geoPolygon, tiles, v, i, &cent))
	}

	WritePointsToCSV(cent, index, filePath[1]+".csv")
	WriteToObj(filePath[1], index, Mesh, v, vn)

}
func SearchIdInGeom(Mesh [][][]Faces, geom []MultiPolygon, tile Tiles, v []Point, i int, cent *[]Point) int {
	const defaultRes = 12030
	res := defaultRes

	// Compute centroid in a single loop
	var p []Point
	var cx, cy float64
	faceCount := len(Mesh[i])

	for _, face := range Mesh[i] {
		vx := v[face[0].v-1]
		cx += vx.X
		cy += vx.Y
		p = append(p, Point{vx.X, vx.Y, 0})
	}

	cx /= float64(faceCount)
	cy /= float64(faceCount)
	point := Point{cx, cy, 0}

	// Search in child tiles
	for _, child := range tile.childTiles {
		if child.extent.minX <= point.X && point.X <= child.extent.maxX &&
			child.extent.minY <= point.Y && point.Y <= child.extent.maxY {

			for _, index := range child.index {
				if IsPointInPolygon(point, geom[index]) {
					*cent = append(*cent, point)
					return index
				}
			}
			for _, index := range child.index {
				for _, pt := range p {
					if IsPointInPolygon(pt, geom[index]) {
						*cent = append(*cent, point)
						return index
					}
				}
			}
		}
	}

	*cent = append(*cent, point)
	return res
}

func CreateTiles(extens Extent, size float64, geom []MultiPolygon) Tiles {

	var tile Tiles
	getExtent := func(points []Point) [4]Point {
		var extent Extent
		var res [4]Point
		for i := 1; i < len(points); i++ {
			GetExtent(points[i].X, points[i].Y, &extent)
		}
		res[0] = Point{extent.minX, extent.maxY, 0}
		res[1] = Point{extent.maxX, extent.maxY, 0}
		res[2] = Point{extent.maxX, extent.minY, 0}
		res[3] = Point{extent.minX, extent.minY, 0}
		return res
	}
	tile.extent = extens
	for w := 0.0; extens.minX+w*size < extens.maxX; w++ {
		for h := 0.0; extens.minY+h*size < extens.maxY; h++ {
			minx := extens.minX + w*size
			maxx := minx + size
			miny := extens.minY + h*size
			maxy := miny + size

			if maxx > extens.maxX {
				maxx = extens.maxX
			}
			if maxy > extens.maxY {
				maxy = extens.maxY
			}

			tileExtent := Extent{maxx, maxy, minx, miny}
			tile.childTiles = append(tile.childTiles, &Tiles{tileExtent, nil, []int{}})
		}
	}

	// for i := 0; i < len(geom); i++ {
	var processPolygon = func(index int, points []Point) {
		if len(points) == 0 {
			return
		}

		extent := getExtent(points)
		for _, extentPoint := range extent {

			for _, child := range tile.childTiles {
				if child.extent.maxX < extentPoint.X || child.extent.minX > extentPoint.X ||
					child.extent.maxY < extentPoint.Y || child.extent.minY > extentPoint.Y {
					continue
				}

				if len(child.index) == 0 || child.index[len(child.index)-1] != index {
					child.index = append(child.index, index)
				}
			}
		}

	}

	for i, g := range geom {
		if len(g.outer) == 0 {
			continue
		}

		processPolygon(i, g.outer)

		for _, island := range g.island {
			processPolygon(i, island.outer)
		}
	}
	// }
	return tile
}
func WriteToObj(baseFilename string, index []int, Mesh [][][]Faces, vertices []Point, normals []Point) {
	// Map untuk menyimpan grup berdasarkan indeks unik
	groupedMeshes := make(map[int][][][]Faces)

	// Kumpulkan semua grup berdasarkan indeks unik
	for i, idx := range index {
		if _, exists := groupedMeshes[idx]; !exists {
			groupedMeshes[idx] = [][][]Faces{} // Inisialisasi jika belum ada
		}
		groupedMeshes[idx] = append(groupedMeshes[idx], Mesh[i])
	}

	// Proses setiap indeks unik dan ekspor sebagai file .obj terpisah
	filePath := strings.Split(baseFilename, "/")
	os.Mkdir("export/"+filePath[len(filePath)-1], os.ModePerm)
	for idx, groups := range groupedMeshes {
		filename := fmt.Sprintf("export/"+filePath[len(filePath)-1]+"/%d.obj", idx)
		file, err := os.Create(filename)
		if err != nil {
			fmt.Println("Error creating file:", err)
			continue
		}
		defer file.Close()
		// Map untuk menyimpan vertex & normal lokal agar indeksnya tetap berurutan
		vertexMap := make(map[int]int)
		normalMap := make(map[int]int)
		localVertices := []Point{}
		localNormals := []Point{}
		vertexCounter := 1
		normalCounter := 1

		// // 1. Kumpulkan semua vertex & normal yang digunakan dalam grup ini
		for _, facesGroup := range groups {
			for _, sides := range facesGroup { // Sisi-sisi dalam grup
				for _, faces := range sides {
					// for _, face := range faces {
					// Konversi indeks vertex ke lokal
					if _, exists := vertexMap[faces.v]; !exists {
						vertexMap[faces.v] = vertexCounter
						localVertices = append(localVertices, vertices[faces.v-1]) // -1 karena index mulai dari 1
						vertexCounter++
					}
					// Konversi indeks normal ke lokal
					if _, exists := normalMap[faces.vn]; !exists {
						normalMap[faces.vn] = normalCounter
						localNormals = append(localNormals, normals[faces.vn-1])
						normalCounter++
					}
					// }
				}
			}
		}

		// 2. Tulis semua vertex (v x y z)
		for _, v := range localVertices {
			file.WriteString(fmt.Sprintf("v %.6f %.6f %.6f\n", v.X, v.Y, v.Z))
		}

		// 3. Tulis semua normal (vn nx ny nz)
		for _, vn := range localNormals {
			file.WriteString(fmt.Sprintf("vn %.6f %.6f %.6f\n", vn.X, vn.Y, vn.Z))
		}

		// 4. Menulis objek dengan nama unik berdasarkan indeks
		file.WriteString(fmt.Sprintf("o Object_%d\n", idx))

		// 5. Menulis face dengan indeks yang sesuai
		for _, facesGroup := range groups {
			for _, sides := range facesGroup { // Sisi dalam grup
				facesTxt := "f "
				for _, face := range sides {

					vLocal := vertexMap[face.v]
					vnLocal := normalMap[face.vn]
					facesTxt += strconv.Itoa(vLocal) + "//" + strconv.Itoa(vnLocal) + " "

				}
				file.WriteString(facesTxt + "\n")
			}
		}
	}
}

func WritePointsToCSV(points []Point, index []int, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	if err := writer.Write([]string{"X", "Y", "Z"}); err != nil {
		return err
	}

	// Write each point to CSV
	Cx := 700621.357389
	Cy := 9311966.06841
	for i, p := range points {
		row := []string{
			strconv.FormatFloat(p.X+Cx, 'f', 6, 64),
			strconv.FormatFloat(p.Y+Cy, 'f', 6, 64),
			strconv.FormatFloat(p.Z, 'f', 6, 64),
			strconv.FormatInt(int64(index[i]), 10),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	fmt.Println("CSV file saved:", filename)

	return nil
}

func IsPointInPolygon(point Point, polygon MultiPolygon) bool {
	const eps = 1e-9
	inside := false
	var queryPolygon = func(inside *bool, polygon MultiPolygon) {
		// for _, ring := range polygon.outer {
		ring := polygon.outer
		n := len(ring)
		if n < 3 {
			*inside = false // Skip invalid polygon parts
		}

		j := n - 1 // Previous vertex index
		for i := 0; i < n; i++ {
			yi, yj := ring[i].Y, ring[j].Y
			if (yi > point.Y+eps) != (yj > point.Y+eps) { // Check y-bounds
				xi, xj := ring[i].X, ring[j].X
				xIntersect := (xj-xi)*(point.Y-yi)/(yj-yi+eps) + xi
				if point.X < xIntersect+eps {
					*inside = !*inside
				}
			}
			j = i
		}
		// }
	}
	queryPolygon(&inside, polygon)
	if !inside {
		for _, island := range polygon.island {
			queryPolygon(&inside, *island)
			if inside {
				return inside
			}

		}
	}

	return inside
}

func ReadMesh(data []byte) ([]Point, []Point, [][][]Faces) {
	var v = []Point{}
	var vn = []Point{}
	var Mesh [][][]Faces
	var err error
	groupIndex := []int{}
	for i := 0; i < len(data)-2; i++ {

		if bytes.Equal(data[0+i:2+i], []byte{10, 111}) {
			groupIndex = append(groupIndex, 0+i)
		}
	}
	// fmt.Println(data)
	for i := 0; i < len(data)-5; i++ {
		if bytes.Equal(data[0+i:5+i], []byte{13, 10, 13, 10, 103}) {
			groupIndex = append(groupIndex, 0+i)
		}
	}
	for i := 0; i < len(groupIndex); i++ {
		group := []byte{}
		if i != len(groupIndex)-1 {
			group = data[groupIndex[i]:groupIndex[i+1]]
		} else {
			group = data[groupIndex[i]:]
		}

		groupSplit := strings.Split(string(group), "\n")
		var meshGroup [][]Faces
		for j := 0; j < len(groupSplit); j++ {
			line := strings.Split(strings.TrimSpace(string(groupSplit[j])), " ")
			if len(line) > 1 {
				if line[0] == "v" {
					var vertex Point
					vertex.X, err = strconv.ParseFloat(line[1], 64)
					vertex.Y, err = strconv.ParseFloat(line[2], 64)
					vertex.Z, err = strconv.ParseFloat(line[3], 64)
					v = append(v, vertex)
					if err != nil {
						fmt.Println(err)
					}

				} else if line[0] == "vn" {
					var vertex Point
					vertex.X, err = strconv.ParseFloat(line[1], 64)
					vertex.Y, err = strconv.ParseFloat(line[2], 64)
					vertex.Z, err = strconv.ParseFloat(line[3], 64)
					vn = append(vn, vertex)

				} else if line[0] == "f" {
					var f = make([]Faces, len(line)-1)
					for k := 1; k < len(line); k++ {
						if len(line[k]) > 0 {
							indexes := strings.Split(line[k], "/")

							value, err := strconv.ParseInt(indexes[0], 10, 64)
							f[k-1].v = int(value)
							value, err = strconv.ParseInt(indexes[2], 10, 64)
							f[k-1].vn = int(value)
							if err != nil {
								fmt.Println(err)
							}

						}
					}
					meshGroup = append(meshGroup, f)
				}

			}
		}
		Mesh = append(Mesh, meshGroup)
	}
	return v, vn, Mesh
}
func GetExtent(X float64, Y float64, extents *Extent) {
	if extents.maxX == 0 || extents.minX == 0 {
		extents.maxX = X
		extents.minX = X
	} else {
		if extents.maxX < X {
			extents.maxX = X
		}
		if X < extents.minX {
			extents.minX = X
		}

	}
	if extents.maxY == 0 || extents.minY == 0 {
		extents.maxY = Y
		extents.minY = Y
	} else {
		if extents.maxY < Y {
			extents.maxY = Y
		}
		if Y < extents.minY {
			extents.minY = Y
		}

	}

}

func ReadGeomGeojson(geojson map[string]interface{}) ([]MultiPolygon, Extent) {
	var MultiPolygons []MultiPolygon
	var extents Extent
	features := geojson["features"].([]interface{})
	Cx := 700621.357389
	Cy := 9311966.06841
	fmt.Println(Cx, Cy)
	for _, feature := range features {
		geometry, ok := feature.(map[string]interface{})["geometry"].(map[string]interface{})
		if !ok {
			continue
		}

		coordinates, ok := geometry["coordinates"].([]interface{})
		if !ok || len(coordinates) == 0 {
			MultiPolygons = append(MultiPolygons, MultiPolygon{}) // Append empty MultiPolygon
			continue
		}

		var polygons MultiPolygon

		for idxPolygon, polygon := range coordinates {
			polygonParts, ok := polygon.([]interface{})
			if !ok {
				continue
			}

			for idxPart, part := range polygonParts {
				coord, ok := part.([]interface{})
				if !ok || len(coord) < 3 {
					continue
				}

				LinerRing := make([]Point, len(coord))
				for j := range coord {
					point := coord[j].([]interface{})
					X, Y := point[0].(float64)-Cx, point[1].(float64)-Cy
					LinerRing[j] = Point{X, Y, 0}

					GetExtent(X, Y, &extents)
				}

				if idxPolygon == 0 {
					if idxPart == 0 {
						polygons.outer = LinerRing
					} else {
						polygons.hole = LinerRing
					}
				} else {
					var island MultiPolygon
					if idxPart == 0 {
						island.outer = LinerRing
					} else {
						island.hole = LinerRing
					}
					polygons.island = append(polygons.island, &island)
				}
			}
		}

		MultiPolygons = append(MultiPolygons, polygons)
	}
	// fmt.Println(geomRes)
	return MultiPolygons, extents
}
func ReadFile(filePath string) []byte {
	file, errFile := os.Open(filePath)
	stat, errStat := os.Stat(filePath)
	defer file.Close()
	if errFile != nil {
		log.Fatal(errFile)
	}
	if errStat != nil {
		log.Fatal(errStat)
	}

	fileLength := stat.Size()
	bytesBuffer := make([]byte, fileLength)
	bin, err := file.Read(bytesBuffer)
	if err != nil {
		log.Fatal(err)
	}
	var data []byte = bytesBuffer[:bin]
	return data
}
