#!/usr/bin/env python3
"""Dummy "system under test": a canned Q&A responder, keyed by task id.

Stands in for a real system (a RAG pipeline, an agent, ...) whose
answers should mention certain keywords - graded by grader_keyword.py
in this same folder.
"""
import json
import sys

ANSWERS = {
    "t1": "The capital of France is Paris.",
    "t2": "The largest planet in the solar system is Jupiter.",
    "t3": "The speed of light is about 299,792,458 meters per second.",
}


def main():
    req = json.loads(sys.stdin.readline())
    task = req["task"]
    output = ANSWERS.get(task["id"], "I don't know.")
    transcript = {"task_id": task["id"], "output": output}
    print(json.dumps(transcript))


if __name__ == "__main__":
    main()
