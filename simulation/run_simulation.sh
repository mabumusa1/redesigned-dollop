#!/bin/bash
# Activate virtual environment and run the simulation
cd "$(dirname "$0")"
source venv/bin/activate
python simulate_match.py