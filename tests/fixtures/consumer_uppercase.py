#!/usr/bin/env python3
"""Single-output consumer fixture: echoes task.input uppercased."""
import json
import sys


def main():
    line = sys.stdin.readline()
    req = json.loads(line)
    task = req["task"]
    output = task["input"].upper() if isinstance(task["input"], str) else task["input"]
    transcript = {"task_id": task["id"], "output": output}
    print(json.dumps(transcript))


if __name__ == "__main__":
    main()
