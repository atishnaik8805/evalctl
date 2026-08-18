#!/usr/bin/env python3
"""Trajectory-aware custom grader fixture: inspects transcript.steps and
implements the custom-grader half of contracts/stdio-protocol.md. When the
transcript carries no steps, it reports the "no trajectory data" failure
per data-model.md's Transcript "Derived state" rules."""
import json
import sys


def main():
    line = sys.stdin.readline()
    req = json.loads(line)
    task = req["task"]
    transcript = req["transcript"]
    steps = transcript.get("steps") or []

    if not steps:
        outcome = {
            "task_id": task["id"],
            "grader_name": "grader_trajectory",
            "score": 0.0,
            "passed": False,
            "error": "no trajectory data",
        }
        print(json.dumps(outcome))
        return

    outcome = {
        "task_id": task["id"],
        "grader_name": "grader_trajectory",
        "score": 1.0,
        "passed": True,
        "evidence": {"step_count": len(steps)},
    }
    print(json.dumps(outcome))


if __name__ == "__main__":
    main()
