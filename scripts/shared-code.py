#!/usr/bin/env python3
"""Conservative first-party source line count, excluding generated scaffolding."""
from pathlib import Path
ROOT = Path(__file__).resolve().parents[1]
shared = specific = 0
for directory in ('frontend/src', 'backend'):
    for path in (ROOT / directory).rglob('*'):
        if path.suffix not in ('.go', '.ts', '.tsx', '.css', '.sql'):
            continue
        if 'ui' in path.parts or 'data' in path.parts or path.name.endswith(('_test.go', '.test.tsx')):
            continue
        content = path.read_text()
        if path.name == 'index.css':
            # Original shadcn generated theme/base scaffold precedes our palette.
            content = content[content.index(':root { --background: #f5f8fa;'):]
        lines = sum(bool(line.strip()) and not line.lstrip().startswith(('//', '--')) for line in content.splitlines())
        if path.name == 'dialect.go' or path.suffix == '.sql':
            specific += lines
        else:
            shared += lines
            if path.name == 'classes.go' and path.parent.name == 'database':
                # The separately authorized booking strategy is engine-specific;
                # its read/insert statements and fixture lifecycle are shared.
                strategy = content[content.index('func (s *store) Reserve('):content.index('func (s *store) reserve(')]
                strategy_lines = sum(bool(line.strip()) and not line.lstrip().startswith('//') for line in strategy.splitlines()) + 3  # FOR UPDATE branch
                shared -= strategy_lines
                specific += strategy_lines
ratio = shared / (shared + specific)
print(f'Shared: {shared}; engine-specific: {specific}; shared ratio: {ratio:.2%}')
assert ratio >= .9, 'Refactor persistence before shipping: less than 90% shared code'
