#!/usr/bin/env python3
"""Custom grader fixture (non-trajectory): checks transcript.output against
task.metadata.expected, implementing the custom-grader half of
contracts/stdio-protocol.md."""
import json
import sys


def main():
    line = sys.stdin.readline()
    req = json.loads(line)
    task = req["task"]
    transcript = req["transcript"]
    expected = (task.get("metadata") or {}).get("expected")
    matched = transcript.get("output") == expected
    outcome = {
        "task_id": task["id"],
        "grader_name": "grader_custom_exact",
        "score": 1.0 if matched else 0.0,
        "passed": matched,
    }
    print(json.dumps(outcome))


if __name__ == "__main__":
    main()
