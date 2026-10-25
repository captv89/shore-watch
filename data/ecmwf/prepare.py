"""Cut the fetched ECMWF fields down to a small grid for one scene.

Reads raw/<time>-wind.grib2 and raw/<time>-wave.grib2 (see fetch.py), or
with --run the steps of one forecast run, raw/run-<run>/<step>h-*.grib2, and
writes one JSON file with 10 m wind, significant wave height and mean wave
direction on the 0.25° grid, inside a box and a time window. The Go tool
weather_on_track and the page's wind layer both read this file, so the
arrows on the chart are the same data the agent's tool used.

Usage:
  uv run --with eccodes --with numpy data/ecmwf/prepare.py \
      --box 48.5,53.5,-22,5 --from 2025-01-21T00 --to 2025-01-24T12 \
      data/weather/s2-tormar-spring.json

  uv run --with eccodes --with numpy data/ecmwf/prepare.py \
      --run 2025-01-24T00 --every 2 --box 42,54,-68,-14 \
      --from 2025-01-24T12 --to 2025-01-31T12 \
      data/weather/s2-forecast-20250124T00.json

Batch mode reads one forecast run once and cuts many boxes from it, as a
JSON list of {"out", "box": [lat_min, lat_max, lon_min, lon_max], "from",
"to"} (the fleet-week; Go writes the list):
  uv run --with eccodes --with numpy data/ecmwf/prepare.py \
      --run 2025-01-20T00 --every 2 --batch data/weather/fleetweek/jobs-20250120T00.json
"""

import argparse
import datetime as dt
import json
import pathlib

import eccodes
import numpy as np

RAW = pathlib.Path(__file__).parent / "raw"
STEP = 0.25
MISSING = 9000  # eccodes marks missing wave values (land, sea ice) as 9999


def read(fn):
    out = {}
    with open(fn, "rb") as f:
        while (g := eccodes.codes_grib_new_from_file(f)) is not None:
            try:
                assert eccodes.codes_get(g, "Ni") == 1440 and eccodes.codes_get(g, "Nj") == 721
                assert eccodes.codes_get(g, "latitudeOfFirstGridPointInDegrees") == 90
                assert eccodes.codes_get(g, "longitudeOfFirstGridPointInDegrees") == 180
                out[eccodes.codes_get(g, "shortName")] = eccodes.codes_get_values(g).reshape(721, 1440)
            finally:
                eccodes.codes_release(g)
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--box", help="lat_min,lat_max,lon_min,lon_max")
    ap.add_argument("--from", dest="first", type=dt.datetime.fromisoformat)
    ap.add_argument("--to", dest="last", type=dt.datetime.fromisoformat)
    ap.add_argument("--batch", type=pathlib.Path, help="with --run: a JSON list of boxes to cut")
    ap.add_argument("--run", type=dt.datetime.fromisoformat, help="read the steps of this forecast run")
    ap.add_argument("--every", type=int, default=1, help="keep every n-th grid point (2 gives 0.5°)")
    ap.add_argument("out", type=pathlib.Path, nargs="?")
    args = ap.parse_args()
    if args.batch:
        batch(args)
        return
    lat0, lat1, lon0, lon1 = map(float, args.box.split(","))

    # Rows run north to south from 90°N; columns run east from 180°, which
    # in -180..180 is -180, so column j is longitude -180 + j * 0.25.
    rows = range(round((90 - lat1) / STEP), round((90 - lat0) / STEP) + 1, args.every)
    cols = range(round((lon0 + 180) / STEP), round((lon1 + 180) / STEP) + 1, args.every)
    idx = np.ix_(list(rows), list(cols))

    if args.run:
        # Every step of the run whose valid time falls in the window.
        run_dir = RAW / f"run-{args.run:%Y%m%dT%H}"
        steps = sorted(int(f.name[:3]) for f in run_dir.glob("*h-wind.grib2"))
        files = [(args.run + dt.timedelta(hours=s), run_dir / f"{s:03d}h") for s in steps]
        files = [(t, f) for t, f in files if args.first <= t <= args.last]
        source = f"ECMWF IFS open data, 0.25°: oper+wave forecast run {args.run:%Y-%m-%d %H}z, steps {steps[0]}-{steps[-1]} h"
    else:
        files, t = [], args.first
        while t <= args.last:
            files.append((t, RAW / f"{t:%Y%m%dT%H}"))
            t += dt.timedelta(hours=3)
        source = "ECMWF IFS open data, 0.25°: 00z/12z oper+wave and 06z/18z scda+scwv runs, steps 0 and 3 h"

    times, fields = [], {"u10": [], "v10": [], "swh": [], "mwd": []}
    for t, f in files:
        wind = read(f"{f}-wind.grib2")
        wave = read(f"{f}-wave.grib2")
        times.append(t.strftime("%Y-%m-%dT%H:%M:%SZ"))
        fields["u10"].append(np.round(wind["10u"][idx], 1))
        fields["v10"].append(np.round(wind["10v"][idx], 1))
        for k in ("swh", "mwd"):
            a = wave[k][idx]
            a = np.where(a > MISSING, np.nan, a)
            fields[k].append(np.round(a, 2 if k == "swh" else 0))

    def tolist(stack):
        return [[[None if np.isnan(x) else float(x) for x in row] for row in grid] for grid in stack]

    doc = {
        "source": source,
        "licence": "CC BY 4.0. Adapted from ECMWF IFS open data (© 2025 ECMWF): cropped and converted to JSON.",
        "attribution": "Based on data and products of the European Centre for Medium-Range Weather Forecasts (ECMWF).",
        "lat_north": lat1,
        "lon_west": lon0,
        "step_deg": STEP * args.every,
        "rows": len(rows),
        "cols": len(cols),
        "times": times,
        "units": {"u10": "m/s", "v10": "m/s", "swh": "m", "mwd": "degrees, direction waves come from"},
        **{k: tolist(v) for k, v in fields.items()},
    }
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(doc, separators=(",", ":"), allow_nan=False) + "\n")
    print(f"{args.out}: {len(times)} times, {len(rows)}x{len(cols)} points, {args.out.stat().st_size // 1024} KB")


def batch(args):
    """Cut every box in the batch file from one run, reading each step once."""
    run_dir = RAW / f"run-{args.run:%Y%m%dT%H}"
    steps = sorted(int(f.name[:3]) for f in run_dir.glob("*h-wind.grib2"))
    source = f"ECMWF IFS open data, 0.25°: oper+wave forecast run {args.run:%Y-%m-%d %H}z, steps {steps[0]}-{steps[-1]} h"
    jobs = json.loads(args.batch.read_text())
    for j in jobs:
        j["first"] = dt.datetime.fromisoformat(j["from"].replace("Z", ""))
        j["last"] = dt.datetime.fromisoformat(j["to"].replace("Z", ""))
        lat0, lat1, lon0, lon1 = j["box"]
        rows = range(round((90 - lat1) / STEP), round((90 - lat0) / STEP) + 1, args.every)
        cols = range(round((lon0 + 180) / STEP), round((lon1 + 180) / STEP) + 1, args.every)
        j["rows"], j["cols"], j["idx"] = rows, cols, np.ix_(list(rows), list(cols))
        j["times"], j["fields"] = [], {"u10": [], "v10": [], "swh": [], "mwd": []}
    for s in steps:
        t = args.run + dt.timedelta(hours=s)
        todo = [j for j in jobs if j["first"] <= t <= j["last"]]
        if not todo:
            continue
        wind = read(run_dir / f"{s:03d}h-wind.grib2")
        wave = read(run_dir / f"{s:03d}h-wave.grib2")
        for j in todo:
            idx = j["idx"]
            j["times"].append(t.strftime("%Y-%m-%dT%H:%M:%SZ"))
            j["fields"]["u10"].append(np.round(wind["10u"][idx], 1))
            j["fields"]["v10"].append(np.round(wind["10v"][idx], 1))
            for k in ("swh", "mwd"):
                a = wave[k][idx]
                a = np.where(a > MISSING, np.nan, a)
                j["fields"][k].append(np.round(a, 2 if k == "swh" else 0))

    def tolist(stack):
        return [[[None if np.isnan(x) else float(x) for x in row] for row in grid] for grid in stack]

    for j in jobs:
        lat0, lat1, lon0, lon1 = j["box"]
        doc = {
            "source": source,
            "licence": "CC BY 4.0. Adapted from ECMWF IFS open data (© 2025 ECMWF): cropped and converted to JSON.",
            "attribution": "Based on data and products of the European Centre for Medium-Range Weather Forecasts (ECMWF).",
            "lat_north": lat1,
            "lon_west": lon0,
            "step_deg": STEP * args.every,
            "rows": len(j["rows"]),
            "cols": len(j["cols"]),
            "times": j["times"],
            "units": {"u10": "m/s", "v10": "m/s", "swh": "m", "mwd": "degrees, direction waves come from"},
            **{k: tolist(v) for k, v in j["fields"].items()},
        }
        out = pathlib.Path(j["out"])
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(json.dumps(doc, separators=(",", ":"), allow_nan=False) + "\n")
    print(f"{args.batch}: {len(jobs)} boxes")


if __name__ == "__main__":
    main()
