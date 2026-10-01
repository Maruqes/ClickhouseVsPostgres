#!/usr/bin/env python3
"""Exercise analytics contract checks on the running six-service stack."""
import json
import subprocess
from datetime import date, datetime, timedelta
from decimal import Decimal
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import urlopen

ROOT = Path(__file__).resolve().parents[1]
MUSCLES = ['back', 'shoulder', 'legs', 'chest', 'tricep', 'bicep', 'forearm']


def docker(*args):
    return subprocess.check_output(['docker', *args], cwd=ROOT, text=True).strip()


def get(base, path):
    try:
        response = urlopen(base + path, timeout=40)
    except HTTPError as error:
        response = error
    with response:
        assert response.headers['Cache-Control'] == 'no-store'
        assert response.headers.get_content_type() == 'application/json'
        return response.status, json.load(response)


def raw_sql(engine, query):
    if engine == 'ch':
        return docker('compose', 'exec', '-T', 'ch-db', 'clickhouse-client', '--user', 'app', '--password', 'app', '--database', 'app', '--query', query, '--format', 'TabSeparated')
    return docker('compose', 'exec', '-T', 'ps-db', 'psql', '-U', 'app', '-d', 'app', '-A', '-t', '-F', '\t', '-c', query)


def main():
    config = json.loads(docker('compose', 'config', '--format', 'json'))
    def base(service):
        return 'http://127.0.0.1:' + str(config['services'][service]['ports'][0]['published'])
    ch, ps = base('ch-front'), base('ps-front')
    metadata = get(ch, '/api/dataset')[1]
    for address in (ch, ps, base('ch-back'), base('ps-back')):
        assert get(address, '/api/dataset') == (200, metadata)
    assert metadata['muscles'] == MUSCLES
    assert metadata['version'] == 'exercise-v2'
    # Whole-dataset checksums and sampled raw records protect against seeding
    # differences, independent of country analytics response shaping.
    checksum = 'SELECT COUNT(*), COUNT(DISTINCT id), SUM(id), SUM(athlete_id), SUM(reps), SUM(CAST(kg * 100 AS BIGINT)) FROM exercises'
    actual = raw_sql('ch', checksum)
    assert actual == raw_sql('ps', checksum), 'Raw dataset checksums differ'
    counts = actual.split('\t')
    assert int(counts[0]) == int(counts[1]) == metadata['rows']
    ids = sorted({i for i in (1, 2, 3, 7, 99, 100, 100000, 250000, 250001, 500001, metadata['rows']) if i <= metadata['rows']})
    sample_query = f"SELECT id, athlete_id, exercise_id, muscle_area, reps, CAST(kg * 100 AS BIGINT), country_code, created_at FROM exercises WHERE id IN ({','.join(map(str, ids))}) ORDER BY id"
    sample = raw_sql('ch', sample_query)
    assert sample == raw_sql('ps', sample_query), 'Sampled raw sets differ'
    nationalities = [(30,'US'), (45,'BR'), (57,'DE'), (67,'IN'), (75,'GB'), (82,'FR'), (88,'JP'), (93,'AU'), (97,'CA'), (99,'PT'), (100,'ZA')]
    new_countries = 'ES IT NL BE CH AT SE NO DK FI PL CZ RO GR TR UA RU CN KR ID TH VN MY PH PK BD SA AE IL EG MA NG KE MX AR CL CO PE NZ'.split()
    nationalities += [(101+i, code) for i, code in enumerate(new_countries)]
    distribution_query = 'SELECT country_code, COUNT(*), COUNT(DISTINCT athlete_id) FROM exercises GROUP BY country_code ORDER BY country_code'
    distribution = raw_sql('ch', distribution_query)
    assert distribution == raw_sql('ps', distribution_query), 'Country distributions differ'
    if metadata['rows'] >= 100000:
        assert {line.split('\t')[0] for line in distribution.splitlines()} == {code for _, code in nationalities}
        # Every athlete retains exactly one nationality across all their sets.
        nationality_query = 'SELECT COUNT(*) FROM (SELECT athlete_id FROM exercises GROUP BY athlete_id HAVING COUNT(DISTINCT country_code) <> 1) AS invalid_athletes'
        assert raw_sql('ch', nationality_query) == raw_sql('ps', nationality_query) == '0'
    for line in sample.splitlines():
        values = line.split('\t')
        i = int(values[0])
        athlete = (i-1) % 100000 + 1
        code = next(code for limit, code in nationalities if ((athlete-1)*37) % 139 < limit)
        timestamp = datetime(2023,1,1) + timedelta(seconds=(i*7919) % 94694400)
        expected = [str(i),str(athlete),str((i*13)%21+1),MUSCLES[(i*13)%7],str((i*17)%16+5),str(((i*29)%781+20)*25),code,str(timestamp)]
        assert values == expected, (values,expected)
    cases = [('US',metadata['from'],metadata['to']), ('PT','2024-02-28','2024-03-01'), ('ZZ','2023-01-01','2023-01-01'), ('ZA','2025-12-31','2025-12-31')]
    cases += [(country, metadata['from'], metadata['to']) for country in ('ES', 'CN', 'KE', 'MX', 'NZ')]
    for country, first, last in cases:
        query = f'?country={country}&from={first}&to={last}'
        results = {}
        for kind in ('summary', 'detail'):
            path = '/api/country/' + kind + query
            status, left = get(ch, path)
            other_status, right = get(ps, path)
            assert status == other_status == 200, (left,right)
            timings = left.pop('query_ms'), right.pop('query_ms')
            assert min(timings) >= 0
            assert left == right, f'Analytics mismatch: {path}'
            results[kind] = left
            print(f'{country} {kind} {first}..{last}: equal results; CH {timings[0]:.2f} ms, PG {timings[1]:.2f} ms')
        detail = results['detail']
        assert detail['totals'] == results['summary']['totals']
        assert [row['key'] for row in detail['muscles']] == MUSCLES
        expected_days = [str(date.fromisoformat(first) + timedelta(days=i)) for i in range((date.fromisoformat(last)-date.fromisoformat(first)).days+1)]
        assert [row['key'] for row in detail['days']] == expected_days
        for groups in (detail['days'], detail['muscles']):
            assert sum(Decimal(row['volume_kg']) for row in groups) == Decimal(detail['totals']['volume_kg'])
        if country == 'ZZ':
            assert detail['totals'] == {'volume_kg':'0.00','athletes':0,'sets':0,'reps':0}
    for query in ('', '?country=us&from=2023-01-01&to=2023-01-02', '?country=US&from=2024-02-30&to=2024-03-01', '?country=US&country=BR&from=2023-01-01&to=2023-01-02', '?country=US&from=2023-01-02&to=2023-01-01'):
        for kind in ('summary','detail'):
            path = '/api/country/' + kind + query
            left = get(ch,path)
            assert left == get(ps,path) and left[0] == 400, f'Error contract: {path}'
    print('PASS: raw seed parity, exact totals, distinct counts, categories, UTC days, empty results, and errors.')


if __name__ == '__main__':
    main()
