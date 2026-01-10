#!/usr/bin/env python3
"""
Retry script for failed webhook events.

This script reads failed events from the SQLite database and attempts to resend them
to the webhook. Successfully resent events are removed from the database.
"""

import sqlite3
import requests
import json
import time
from datetime import datetime

# Database file for failed events
FAILED_EVENTS_DB = "failed_events.db"

# Webhook URL for sending events
WEBHOOK_URL = "https://71d6f6b300c84269d0cfe79a02a8cb00.m.pipedream.net"

def retry_failed_events(max_retries_per_event=3, delay_between_retries=1):
    """Retry sending failed events to the webhook."""

    conn = sqlite3.connect(FAILED_EVENTS_DB)
    cursor = conn.cursor()

    # Get all failed events
    cursor.execute("SELECT id, event_data, retry_count FROM failed_events ORDER BY created_at")
    failed_events = cursor.fetchall()

    if not failed_events:
        print("No failed events to retry.")
        conn.close()
        return

    print(f"Found {len(failed_events)} failed events to retry.")

    successful_retries = 0
    permanent_failures = 0

    for event_id, event_data_str, retry_count in failed_events:
        if retry_count >= max_retries_per_event:
            print(f"Event {event_id} has exceeded max retries ({max_retries_per_event}), skipping.")
            permanent_failures += 1
            continue

        try:
            event_data = json.loads(event_data_str)

            # Attempt to send the event
            response = requests.post(WEBHOOK_URL, json=event_data, timeout=10)

            if response.status_code == 200:
                print(f"✅ Successfully resent event {event_id}")
                # Remove the event from database
                cursor.execute("DELETE FROM failed_events WHERE id = ?", (event_id,))
                successful_retries += 1
            else:
                print(f"❌ Failed to resend event {event_id}: HTTP {response.status_code}")
                # Increment retry count
                cursor.execute(
                    "UPDATE failed_events SET retry_count = retry_count + 1 WHERE id = ?",
                    (event_id,)
                )
                permanent_failures += 1

        except Exception as e:
            print(f"❌ Failed to resend event {event_id}: {str(e)}")
            # Increment retry count
            cursor.execute(
                "UPDATE failed_events SET retry_count = retry_count + 1 WHERE id = ?",
                (event_id,)
            )
            permanent_failures += 1

        # Small delay between retries to be respectful to the webhook
        time.sleep(delay_between_retries)

    conn.commit()
    conn.close()

    print("\nRetry summary:")
    print(f"✅ Successfully resent: {successful_retries}")
    print(f"❌ Still failed (will retry later): {permanent_failures}")

    # Show remaining failed events
    conn = sqlite3.connect(FAILED_EVENTS_DB)
    cursor = conn.cursor()
    cursor.execute("SELECT COUNT(*) FROM failed_events")
    remaining = cursor.fetchone()[0]
    conn.close()

    if remaining > 0:
        print(f"📊 Remaining failed events in database: {remaining}")
    else:
        print("🎉 All failed events have been successfully resent!")

if __name__ == "__main__":
    print("Starting retry of failed webhook events...")
    print(f"Webhook URL: {WEBHOOK_URL}")
    print(f"Database: {FAILED_EVENTS_DB}")
    print("-" * 50)

    retry_failed_events()

    print("\nRetry process complete.")