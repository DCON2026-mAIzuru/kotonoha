#!/usr/bin/env python3
"""Create a private LLM key file only after the user has connected the runtime."""
import argparse
import os
import secrets
from pathlib import Path
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--runtime-connected', action='store_true', required=True)
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
# O_EXCL preserves any existing credential; no key is printed or passed in argv.
fd = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as output:
    output.write(secrets.token_urlsafe(32) + '\n')
print('Created private key file (0600). The key was not printed.')
