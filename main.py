import os
import json
from glob import glob
import geopandas as gpd
import pandas as pd

# ============================================
# 1. Manual OBJ parser
# ============================================

def parse_obj(filepath,cx,cy):
    vertices = []
    faces = []

    with open(filepath, "r") as f:
        for line in f:
            parts = line.strip().split()
            if not parts:
                continue

            if parts[0] == "v":
                x, y, z = map(float, parts[1:])
                vertices.append([x+cx, y+cy, z])

            elif parts[0] == "f":
                face = []
                for item in parts[1:]:
                    if "//" in item:
                        v_idx = int(item.split("//")[0]) - 1
                    else:
                        v_idx = int(item) - 1
                    face.append(v_idx)
                faces.append(face)

    return vertices, faces


# ============================================
# 2. Convert OBJ faces to CityJSON boundaries
# ============================================

def convert_faces_to_cityjson(faces):
    return [[[face] for face in faces]]   # each face = polygon


# ============================================
# 3. Merge all OBJ geometries
# ============================================

def merge_geometries(obj_map):
    global_vertices = []
    cityobjects = {}

    for idx, (vertices, faces) in obj_map.items():
        idx_shift = len(global_vertices)
        shifted_faces = [[v + idx_shift for v in face] for face in faces]

        global_vertices.extend(vertices)

        cityobjects[str(idx)] = {
            "type": "Building",
            "geometry": [{
                "type": "Solid",
                "boundaries": convert_faces_to_cityjson(shifted_faces)
            }],
        }

    return global_vertices, cityobjects


# ============================================
# 4. Inject attributes based on properties.id
# ============================================


def attach_attributes(cityobjects, gdf):
    geom_col = gdf.geometry.name

    for _, row in gdf.iterrows():
        geo_id = str(row["id"])  # id is string in your data

        if geo_id in cityobjects:
            attrs = row.drop(labels=geom_col).to_dict()

            # Replace NaN / None -> ""
            for k, v in attrs.items():
                if pd.isna(v):
                    attrs[k] = ""
            
            cityobjects[geo_id]["attributes"] = attrs

    return cityobjects


# ============================================
# 5. MAIN PIPELINE
# ============================================

def generate_cityjson(obj_folder, geojson_path,cx,cy, output_path="city.json"):
    # Load geojson
    gdf = gpd.read_file(geojson_path)

    # Parse all OBJ files (filename = {id}.obj)
    obj_files = glob(os.path.join(obj_folder, "*.obj"))
    obj_map = {}

    for file in obj_files:
        base = os.path.basename(file)
        name = os.path.splitext(base)[0]

        # OBJ names are strings but convertible to numbers? No problem — store as string index
        if name.isdigit():
            idx = str(name)
            obj_map[idx] = parse_obj(file,cx,cy)

    # Merge OBJ geometry
    global_vertices, cityobjects = merge_geometries(obj_map)

    # Attach GeoJSON attributes
    cityobjects = attach_attributes(cityobjects, gdf)

    # Build CityJSON
    cityjson = {
        "type": "CityJSON",
        "version": "1.1",
        "CityObjects": cityobjects,
        "vertices": global_vertices,
        "transform": None,
        "metadata": {
            "referenceSystem": gdf.crs.to_string() if gdf.crs else None
        }
    }

    # Save
    with open(output_path, "w") as f:
        json.dump(cityjson, f, indent=2)

    print(f"CityJSON saved to: {output_path}")



# ============================================
# 6. RUN
# ============================================

if __name__ == "__main__":
    OBJ_FOLDER = "export/3D_UJI_MONEV_LOD1.obj"            # folder dimana 1.obj, 2.obj, 3.obj ...
    GEOJSON_PATH = "dataa/BO_Merge_MONEV_UTM_V2.geojson"
    OUTPUT = "merged_city.json"
    cx,cy = 700621.357389,9311966.06841

    generate_cityjson(OBJ_FOLDER, GEOJSON_PATH,cx,cy, OUTPUT)
