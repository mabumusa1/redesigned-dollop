# Football Match Simulator

A Python script that simulates realistic football match events between Al Hilal and Al Nassr teams using their player data.

## Features

- Realistic event generation (20-50 events per minute)
- Multiple event types: passes, shots, goals, fouls, cards, corners, free kicks, interceptions
- Random team formations for each simulation
- Progress bar using tqdm
- JSON-compatible event structure for analytics
- Virtual environment setup for isolated dependencies

## Requirements

- Python 3.6+
- CSV files: `alhilal.csv` and `alnassr.csv` with player data

## Setup

### Option 1: Using existing virtual environment (recommended)
The virtual environment is already set up in the `venv/` folder with all dependencies installed.

### Option 2: Fresh setup
If you need to set up the environment from scratch:

```bash
# Create virtual environment
python3 -m venv venv

# Activate virtual environment
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Verify installation
pip list | grep tqdm
```

## Usage

### Option 1: Using the run script (recommended)
```bash
./run_simulation.sh
```

### Option 2: Manual activation
```bash
source venv/bin/activate
python simulate_match.py
```

## Output

The simulation generates:
- Starting lineups for both teams with random formations
- Minute-by-minute event logs
- Progress bar showing simulation progress
- Summary statistics (total events, events per minute, average)

## Event Structure

Each event follows this base JSON structure:

```json
{
  "eventId": "uuid-string",
  "matchId": "uuid-string",
  "eventType": "event_type",
  "timestamp": "2026-01-10T15:30:00.000000",
  "teamId": 1,
  "playerId": "player_id",
  "metadata": {
    // Event-specific data (see examples below)
  }
}
```

### Event Types and Metadata Examples

#### Pass Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440000",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "pass",
  "timestamp": "2026-01-10T15:30:15.123456",
  "teamId": 1,
  "playerId": "23",
  "metadata": {
    "action": "pass",
    "from_id": "23",
    "to_id": "11"
  }
}
```

#### Shot Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440002",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "shot",
  "timestamp": "2026-01-10T15:31:22.456789",
  "teamId": 2,
  "playerId": "7",
  "metadata": {
    "action": "shot",
    "shooter_id": "7"
  }
}
```

#### Goal Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440003",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "goal",
  "timestamp": "2026-01-10T15:32:45.789012",
  "teamId": 1,
  "playerId": "11",
  "metadata": {
    "action": "goal",
    "scorer_id": "11",
    "assist_id": "23"
  }
}
```

#### Foul Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440004",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "foul",
  "timestamp": "2026-01-10T15:33:12.345678",
  "teamId": 2,
  "playerId": "5",
  "metadata": {
    "action": "foul",
    "fouler_id": "5",
    "victim_id": "18"
  }
}
```

#### Yellow Card Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440005",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "yellow_card",
  "timestamp": "2026-01-10T15:34:08.901234",
  "teamId": 1,
  "playerId": "77",
  "metadata": {
    "action": "card",
    "player_id": "77",
    "card_type": "yellow",
    "reason": "unsporting behavior"
  }
}
```

#### Red Card Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440006",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "red_card",
  "timestamp": "2026-01-10T15:35:55.678901",
  "teamId": 2,
  "playerId": "21",
  "metadata": {
    "action": "card",
    "player_id": "21",
    "card_type": "red",
    "reason": "serious foul play"
  }
}
```

#### Offside Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440007",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "offside",
  "timestamp": "2026-01-10T15:36:33.456789",
  "teamId": 1,
  "playerId": "23",
  "metadata": {
    "action": "offside",
    "player_id": "23"
  }
}
```

#### Corner Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440008",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "corner",
  "timestamp": "2026-01-10T15:37:41.234567",
  "teamId": 2,
  "playerId": "29",
  "metadata": {
    "action": "set_piece",
    "type": "corner",
    "taker_id": "29"
  }
}
```

#### Free Kick Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440009",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "free_kick",
  "timestamp": "2026-01-10T15:38:17.890123",
  "teamId": 1,
  "playerId": "12",
  "metadata": {
    "action": "set_piece",
    "type": "free_kick",
    "taker_id": "12"
  }
}
```

#### Interception Event
```json
{
  "eventId": "550e8400-e29b-41d4-a716-446655440010",
  "matchId": "550e8400-e29b-41d4-a716-446655440001",
  "eventType": "interception",
  "timestamp": "2026-01-10T15:39:29.567890",
  "teamId": 1,
  "playerId": "5",
  "metadata": {
    "action": "interception",
    "interceptor_id": "5",
    "intercepted_from_id": "16"
  }
}
```

## Dependencies

- tqdm: Progress bar library (version 4.67.1)
- Standard Python libraries: csv, random, uuid, json, datetime

All dependencies are listed in `requirements.txt` and managed in the local virtual environment.