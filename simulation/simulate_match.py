import csv
import random
import uuid
import json
from datetime import datetime, timedelta
from tqdm import tqdm

ALHILAL_CSV = "alhilal.csv"
ALNASSR_CSV = "alnassr.csv"

# Team constants
TEAM_AL_HILAL = "Al Hilal"
TEAM_AL_NASSR = "Al Nassr"

# Team ID constants
TEAM_ID_AL_HILAL = 2
TEAM_ID_AL_NASSR = 1

def load_players(csv_path):
    players = []
    with open(csv_path, newline='') as csvfile:
        reader = csv.DictReader(csvfile)
        for row in reader:
            players.append({
                'id': row['id'],
                'name': row['name'],
                'position': row['position'].lower()
            })
    return players

def select_starting_xi(players):
    # Common football formations (excluding GK, always 1)
    formations = [
        {'def': 4, 'mid': 4, 'fwd': 2},  # 4-4-2
        {'def': 4, 'mid': 3, 'fwd': 3},  # 4-3-3
        {'def': 3, 'mid': 5, 'fwd': 2},  # 3-5-2
        {'def': 5, 'mid': 3, 'fwd': 2},  # 5-3-2
        {'def': 4, 'mid': 5, 'fwd': 1},  # 4-5-1
        {'def': 3, 'mid': 4, 'fwd': 3},  # 3-4-3
    ]
    formation = random.choice(formations)
    # Always 1 GK
    full_formation = {'gk': 1}
    full_formation.update(formation)
    selected = []
    for pos, count in full_formation.items():
        pos_players = [p for p in players if p['position'] == pos]
        if len(pos_players) < count:
            raise ValueError(f"Not enough players for position {pos}")
        selected += random.sample(pos_players, count)
    return selected, full_formation


def get_event_metadata(event_type, player, all_players, team_id, team1_id=TEAM_ID_AL_HILAL, team2_id=TEAM_ID_AL_NASSR):
    """Generate structured metadata for different event types."""
    metadata = {"action": event_type}

    # Define event handlers
    event_handlers = {
        "pass": lambda: handle_pass(player, all_players, team_id),
        "shot": lambda: {"shooter_id": player['id']},
        "goal": lambda: handle_goal(player, all_players, team_id),
        "foul": lambda: handle_foul(player, all_players, team_id, team1_id, team2_id),
        "yellow_card": lambda: {"player_id": player['id'], "card_type": "yellow", "reason": "unsporting behavior"},
        "red_card": lambda: {"player_id": player['id'], "card_type": "red", "reason": "serious foul play"},
        "offside": lambda: {"player_id": player['id']},
        "corner": lambda: {"type": "corner", "taker_id": player['id']},
        "free_kick": lambda: {"type": "free_kick", "taker_id": player['id']},
        "interception": lambda: handle_interception(player, all_players, team_id, team1_id, team2_id)
    }

    handler = event_handlers.get(event_type)
    if handler:
        metadata.update(handler())

    return metadata

def handle_pass(player, all_players, team_id):
    """Handle pass event metadata."""
    target = random.choice([p for p in all_players[team_id] if p['id'] != player['id']])
    return {"from_id": player['id'], "to_id": target['id']}

def handle_goal(player, all_players, team_id):
    """Handle goal event metadata."""
    metadata = {"scorer_id": player['id']}
    # 70% chance of having an assist
    if random.random() < 0.7:
        assister = random.choice([p for p in all_players[team_id] if p['id'] != player['id']])
        metadata["assist_id"] = assister['id']
    return metadata

def handle_foul(player, all_players, team_id, team1_id, team2_id):
    """Handle foul event metadata."""
    opponent_team_id = team2_id if team_id == team1_id else team1_id
    victim = random.choice(all_players[opponent_team_id])
    return {"fouler_id": player['id'], "victim_id": victim['id']}

def handle_interception(player, all_players, team_id, team1_id, team2_id):
    """Handle interception event metadata."""
    opponent_team_id = team2_id if team_id == team1_id else team1_id
    intercepted_from = random.choice(all_players[opponent_team_id])
    return {"interceptor_id": player['id'], "intercepted_from_id": intercepted_from['id']}

def create_event(event_type, match_id, minute, team_id, player, all_players, current_time, team1_id=TEAM_ID_AL_HILAL, team2_id=TEAM_ID_AL_NASSR):
    """Create a single event dictionary."""
    event = {
        "eventId": str(uuid.uuid4()),
        "matchId": match_id,
        "eventType": event_type,
        "timestamp": (current_time + timedelta(minutes=minute-1)).isoformat(),
        "teamId": team_id,
        "playerId": player['id'],
        "metadata": get_event_metadata(event_type, player, all_players, team_id, team1_id, team2_id)
    }
    return event

def simulate_events(team1, team2, team1_id=TEAM_ID_AL_HILAL, team2_id=TEAM_ID_AL_NASSR, minutes=5):
    # Event types and their probabilities per minute
    event_types = [
        ("pass", 0.65),
        ("shot", 0.12),
        ("goal", 0.03),
        ("foul", 0.08),
        ("yellow_card", 0.02),
        ("red_card", 0.005),
        ("substitution", 0.0),  # No subs in first 5 min
        ("offside", 0.02),
        ("corner", 0.03),
        ("free_kick", 0.03),
        ("interception", 0.04),  # Player intercepting another player's pass
    ]
    # Normalize probabilities
    total_prob = sum([p for _, p in event_types])
    event_types = [(e, p/total_prob) for e, p in event_types]
    match_id = str(uuid.uuid4())
    all_players = {team1_id: team1, team2_id: team2}
    # Simulate minute by minute
    current_time = datetime.now().replace(second=0, microsecond=0)
    print(f"\nSimulating {minutes} minutes of the match (Match ID: {match_id})\n")

    total_events = 0
    events_per_minute = []

    # Use tqdm for progress bar
    for minute in tqdm(range(1, minutes+1), desc="Simulating match", unit="min"):
        n_events = random.randint(20, 50)  # 20-50 events per minute (realistic football match)
        events_per_minute.append(n_events)
        total_events += n_events

        # Update progress bar description with current stats
        tqdm.write(f"Minute {minute}: {n_events} events (Total: {total_events})")

        events_this_minute = []
        for _ in range(n_events):
            event_type = random.choices([e for e, _ in event_types], [p for _, p in event_types])[0]
            team_id = random.choice([team1_id, team2_id])
            player = random.choice(all_players[team_id])
            event = create_event(event_type, match_id, minute, team_id, player, all_players, current_time, team1_id, team2_id)
            events_this_minute.append(event)

        # Print events for this minute
        print(f"\n  Events in minute {minute}:")
        for event in events_this_minute:
            print(f"    {event['eventType'].upper()}: Team {event['teamId']} - Player {event['playerId']}")

    print(f"\nSimulation complete! Total events: {total_events}")
    print(f"Events per minute: {events_per_minute}")
    print(f"Average events per minute: {total_events/minutes:.1f}")

    # Optionally return all events for further processing
    return []  # Could return all events if needed

def main():
    team1_players = load_players(ALHILAL_CSV)
    team2_players = load_players(ALNASSR_CSV)
    team1_xi, team1_formation = select_starting_xi(team1_players)
    team2_xi, team2_formation = select_starting_xi(team2_players)
    print("Al Hilal Starting XI (Formation: GK-{def}-{mid}-{fwd}):".format(**team1_formation))
    for p in team1_xi:
        print(f"  {p['name']} ({p['position'].upper()})")
    print("\nAl Nassr Starting XI (Formation: GK-{def}-{mid}-{fwd}):".format(**team2_formation))
    for p in team2_xi:
        print(f"  {p['name']} ({p['position'].upper()})")
    simulate_events(team1_xi, team2_xi, TEAM_ID_AL_HILAL, TEAM_ID_AL_NASSR, minutes=5)

if __name__ == "__main__":
    main()