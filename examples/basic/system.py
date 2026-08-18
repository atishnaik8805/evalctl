#!/usr/bin/env python3
"""Dummy "system under test": reads one Task from stdin, writes one
Transcript to stdout, uppercasing task.input.

This stands in for whatever real system you want to evaluate (an LLM
agent, a chatbot, a pipeline, ...) - evalctl only cares that it speaks
the stdio protocol in specs/001-universal-eval-framework/contracts/stdio-protocol.md.
"""
import json
import sys


def main():
    req = json.loads(sys.stdin.readline())
    task = req["task"]
    output = task["input"].upper()
    transcript = {"task_id": task["id"], "output": output}
    print(json.dumps(transcript))


if __name__ == "__main__":
    main()
