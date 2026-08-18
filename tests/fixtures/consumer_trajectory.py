#!/usr/bin/env python3
"""Trajectory consumer fixture: returns steps alongside output."""
import json
import sys


def main():
    line = sys.stdin.readline()
    req = json.loads(line)
    task = req["task"]
    steps = [
        {"index": 0, "state": {"start": True}, "action": "search", "result": {"found": task["input"]}},
        {"index": 1, "state": {"found": task["input"]}, "action": "answer", "result": {"answer": task["input"]}},
    ]
    transcript = {"task_id": task["id"], "output": task["input"], "steps": steps}
    print(json.dumps(transcript))


if __name__ == "__main__":
    main()
