"""Fetch ECMWF Open Data fields for the Scene 2 weather week.

Downloads only the fields we need (10 m wind, wave height and direction) by
HTTP range request, using the .index file next to each GRIB2 file. Raw GRIB
stays in raw/ (gitignored); prepare.py turns it into small JSON for the page.

Source: s3://ecmwf-forecasts (public, no account). Licence CC-BY-4.0.

Runs 00z and 12z are the main runs (oper, wave). Runs 06z and 18z are the
short-cutoff runs (scda, scwv). Steps 0 and 3 from all four give 3-hourly
fields with a lead time of at most 3 h: the weather as known at the time.

Usage: uv run --with requests data/ecmwf/fetch.py 2025-01-17 2025-01-30 [--steps 0,3]

Forecast mode fetches one run's steps ahead, for looking forward from a
report time. Files go to raw/run-<run>/<step>h-{wind,wave}.grib2:
  uv run --with requests data/ecmwf/fetch.py --run 2025-01-24T00 --steps 12-180

The fleet-week fetches one run a day, 6-hourly to 120 h:
  uv run --with requests data/ecmwf/fetch.py --run 2025-01-20T00 --steps 0-120/6
"""

import argparse
import datetime as dt
import json
import pathlib
import sys
import time

import requests

BUCKET = "https://ecmwf-forecasts.s3.eu-central-1.amazonaws.com"
STREAMS = {  # run hour -> (atmosphere stream, wave stream)
    0: ("oper", "wave"),
    6: ("scda", "scwv"),
    12: ("oper", "wave"),
    18: ("scda", "scwv"),
}
ATMOS_PARAMS = {"10u", "10v"}
WAVE_PARAMS = {"swh", "mwd"}
RAW = pathlib.Path(__file__).parent / "raw"


def get(session, url, **kw):
    """GET with backoff: S3 answers 503 "Slow Down" under load."""
    for attempt in range(9):
        time.sleep(0.2)  # stay under the bucket's request rate
        r = session.get(url, timeout=120, **kw)
        if r.status_code not in (500, 503):
            r.raise_for_status()
            return r
        time.sleep(min(2**attempt, 60))
    r.raise_for_status()


def fetch_fields(session, run, hour, stream, step, params, out):
    stem = f"{run:%Y%m%d}/{hour:02d}z/ifs/0p25/{stream}/{run:%Y%m%d}{hour:02d}0000-{step}h-{stream}-fc"
    idx = get(session, f"{BUCKET}/{stem}.index")
    entries = [json.loads(line) for line in idx.text.splitlines() if line.strip()]
    wanted = [e for e in entries if e["param"] in params]
    missing = params - {e["param"] for e in wanted}
    if missing:
        raise RuntimeError(f"{stem}: missing {sorted(missing)}")
    tmp = out.with_suffix(".part")
    with tmp.open("wb") as f:
        for e in wanted:
            start, length = e["_offset"], e["_length"]
            r = get(session, f"{BUCKET}/{stem}.grib2", headers={"Range": f"bytes={start}-{start + length - 1}"})
            f.write(r.content)
    tmp.rename(out)


def forecast_steps(spec):
    """"12-180" -> the run's steps in that range: 3-hourly to 144 h, then
    6-hourly. "0-120/6" keeps only the steps a multiple of 6 h."""
    spec, _, every = spec.partition("/")
    lo, hi = map(int, spec.split("-"))
    available = list(range(0, 145, 3)) + list(range(150, 361, 6))
    return [s for s in available if lo <= s <= hi and s % int(every or 1) == 0]


def fetch_run(session, run, steps):
    atmos, wave = STREAMS[run.hour]
    out_dir = RAW / f"run-{run:%Y%m%dT%H}"
    out_dir.mkdir(parents=True, exist_ok=True)
    for step in steps:
        for kind, stream, params in (("wind", atmos, ATMOS_PARAMS), ("wave", wave, WAVE_PARAMS)):
            out = out_dir / f"{step:03d}h-{kind}.grib2"
            if out.exists():
                continue
            fetch_fields(session, run.date(), run.hour, stream, step, params, out)
            print(out.relative_to(RAW), file=sys.stderr)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("first", type=dt.date.fromisoformat, nargs="?")
    ap.add_argument("last", type=dt.date.fromisoformat, nargs="?")
    ap.add_argument("--steps", default="0,3")
    ap.add_argument("--run", type=dt.datetime.fromisoformat, help="forecast mode: one run, e.g. 2025-01-24T00")
    args = ap.parse_args()

    RAW.mkdir(exist_ok=True)
    if args.run:
        fetch_run(requests.Session(), args.run, forecast_steps(args.steps))
        return
    steps = [int(s) for s in args.steps.split(",")]

    RAW.mkdir(exist_ok=True)
    session = requests.Session()
    day = args.first
    while day <= args.last:
        for hour, (atmos, wave) in STREAMS.items():
            for step in steps:
                valid = dt.datetime(day.year, day.month, day.day, hour) + dt.timedelta(hours=step)
                for kind, stream, params in (("wind", atmos, ATMOS_PARAMS), ("wave", wave, WAVE_PARAMS)):
                    out = RAW / f"{valid:%Y%m%dT%H}-{kind}.grib2"
                    if out.exists():
                        continue
                    fetch_fields(session, day, hour, stream, step, params, out)
                    print(out.name, file=sys.stderr)
        day += dt.timedelta(days=1)


if __name__ == "__main__":
    main()
