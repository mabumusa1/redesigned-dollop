# Football Match Simulator

A Python script that simulates realistic football match events between Al Hilal and Al Nassr teams using their player data, with webhook integration and automatic retry for failed events.

## Features

- Realistic event generation (20-50 events per minute)
- Multiple event types: passes, shots, goals, fouls, cards, corners, free kicks, interceptions
- Random team formations for each simulation
- Progress bar using tqdm
- JSON-compatible event structure for analytics
- **Webhook integration**: Events are sent to a configurable webhook URL in real-time
- **Failed events handling**: Failed webhook deliveries are automatically saved to SQLite database
- **Retry mechanism**: Separate script to retry failed events with configurable retry limits
- Virtual environment setup for isolated dependencies

## Requirements

- Python 3.6+
- CSV files: `alhilal.csv` and `alnassr.csv` with player data

## Setup

### Virtual Environment Setup

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

### Webhook Configuration

The simulation sends events to a webhook URL. The default webhook URL is configured in `simulate_match.py`:

```python
WEBHOOK_URL = "https://71d6f6b300c84269d0cfe79a02a8cb00.m.pipedream.net"
```

You can modify this URL in the script to point to your own webhook endpoint.

## Usage

### Running the Simulation

First, ensure the virtual environment is set up and activated:

```bash
# Activate virtual environment
source venv/bin/activate
```

### Option 1: Using the run script
```bash
./run_simulation.sh
```

### Option 2: Manual execution
```bash
python simulate_match.py
```

### Retrying Failed Events

If some events failed to send during simulation, you can retry them using the retry script:

```bash
python retry_failed_events.py
```

The retry script will:
- Read failed events from the SQLite database
- Attempt to resend them to the webhook
- Remove successfully resent events from the database
- Respect retry limits (default: 3 retries per event)

## Output

The simulation generates:
- Starting lineups for both teams with random formations
- Minute-by-minute event logs
- Progress bar showing simulation progress
- Real-time webhook delivery status (success/failure counts)
- Summary statistics (total events, events per minute, average)
- Failed events are automatically saved to `failed_events.db` for later retry

### Webhook Delivery

Each event is sent to the configured webhook URL as a JSON payload. The simulation tracks:
- ✅ Successfully sent events
- ❌ Failed events (saved to database with error details)

### Database

Failed events are stored in `failed_events.db` with the following structure:
- `id`: Auto-incrementing primary key
- `event_data`: JSON string of the failed event
- `failure_reason`: Description of why the event failed
- `created_at`: Timestamp when the failure occurred
- `retry_count`: Number of retry attempts (default: 0)

## Retry Failed Events

The `retry_failed_events.py` script provides functionality to retry events that failed to send during simulation:

### Features
- Reads failed events from the SQLite database
- Attempts to resend events to the webhook with configurable retry limits
- Removes successfully resent events from the database
- Provides detailed logging of retry attempts and results
- Respects maximum retry count per event (default: 3 retries)

### Configuration
- **Database**: `failed_events.db` (same as simulation)
- **Webhook URL**: Configurable in the script (same as simulation)
- **Max retries per event**: Configurable (default: 3)
- **Delay between retries**: Configurable (default: 1 second)

### Usage
```bash
python retry_failed_events.py
```

### Output
The retry script provides:
- Count of failed events found
- Real-time status of each retry attempt
- Summary of successful vs failed retries
- Remaining failed events count

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
- requests: HTTP library for webhook communication
- Standard Python libraries: csv, random, uuid, json, datetime, sqlite3

All dependencies are listed in `requirements.txt` and managed in the local virtual environment.