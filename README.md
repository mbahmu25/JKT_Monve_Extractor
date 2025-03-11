# JKT Monev Extractor

OBJ Data extractor untuk keperluan 3D City Building, dapat mengakomodir tingkat kedetailan LOD1 - LOD3
Dengan catatan memiliki data OBJ dan GeoJSON untuk Building Outline
## Petnjuk Penggunaan
#### Setup Module 
```go
go mod download
```
### Build and Compile App
```go
go build .
```
Output akan berupa file exe dengan nama objExtractor.exe
### Run sysntax
#### Windows
```bash
objExtractor [file path OBJ] [file path BO GeoJSON]
```
#### Linux
```bash
./objExtractor [file path OBJ] [file path BO GeoJSON]
```

Output akan terdiri dari CSV dan folder OBJ
