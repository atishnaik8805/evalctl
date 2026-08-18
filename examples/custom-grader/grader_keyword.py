#!/usr/bin/env python3
"""Custom grader (contracts/stdio-protocol.md, custom-grader role): reads
a {task, transcript} request from stdin, scores the fraction of
task.metadata.keywords that appear as substrings of transcript.output,
and writes an Outcome to stdout.

Custom graders are plain executables - evalctl never imports this code,
it only runs it as a subprocess per the stdio protocol. Any language
that can read a line of JSON from stdin and write one to stdout works.
"""
import json
import sys


def main():
    req = json.loads(sys.stdin.readline())
    task = req["task"]
    transcript = req["transcript"]
    keywords = (task.get("metadata") or {}).get("keywords", [])
    output = transcript.get("output", "")

    found = [k for k in keywords if k in output]
    score = len(found) / len(keywords) if keywords else 0.0
    outcome = {
        "task_id": task["id"],
        "grader_name": "grader_keyword",
        "score": score,
        "passed": len(found) == len(keywords),
        "evidence": {"matched_keywords": found},
    }
    print(json.dumps(outcome))


if __name__ == "__main__":
    main()
