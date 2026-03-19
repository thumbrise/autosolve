# autosolve

[![CI](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml/badge.svg)](https://github.com/thumbrise/autosolve/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/thumbrise/autosolve.svg)](https://pkg.go.dev/github.com/thumbrise/autosolve)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](/LICENSE)

Available languages: [English](/README.md), [Русский](/docs/README.ru.md)

# GitHub Dispatcher Agent – automatic AI assistant for your repositories

## Idea

We are building a simple and reliable tool that automatically monitors activity in a GitHub repository and, when needed, launches AI agents to solve tasks such as bug fixes, feature additions, or code analysis. The product is a standalone process that works on a polling basis, requires no complex webhook or CI/CD setup, and can be deployed anywhere – from a personal laptop to a dedicated server.

## Problem

Modern AI tools for development automation (OpenHands, SWE-agent, Devin) have powerful capabilities, but their integration with GitHub is often complex or unstable:

- they require setting up webhooks or GitHub Actions, which is not always convenient
- built-in schedulers (cron) are unreliable or need extra infrastructure
- most solutions are designed for one-off runs on demand, not for continuous background monitoring
- for small teams or individual developers, deploying such agents can be too complicated and expensive

## Our solution

We propose a lightweight dispatcher that:

- runs as a persistent process (daemon) on your computer or server
- periodically (e.g., every 5–10 minutes) polls the GitHub API for a given repository
- detects new issues, pull requests, comments, mentions, and labels
- stores the state of the last check in local storage (the exact technology choice is open – it could be a file, an embedded database, or something else)
- when events matching predefined rules occur (e.g., an issue with the label `ai` or a comment containing `@bot`), it invokes an external AI tool (like ra-aid) and passes the task context
- after the AI finishes, it publishes the result back to GitHub – posting a comment, creating a pull request, or performing another action

All decisions about specific technologies (programming language, storage type, deployment method) can be made later, as we better understand the requirements. The architecture is designed so that components can be replaced without rewriting the whole system.

## How it helps users

- **Simplicity**: no need to understand webhooks or GitHub Actions – just run it and forget
- **Reliability**: polling with state persistence ensures no event is missed, even after restarts
- **Flexibility**: any AI tool that can be invoked from the command line can be used (ra-aid, local models via Ollama, Python scripts, etc.)
- **Resource efficiency**: the agent consumes minimal CPU and memory, and can run even on a Raspberry Pi
- **Security**: all data stays with you; tokens are not shared with third parties

## High-level architecture

The system consists of several logical blocks that can be implemented in different ways:

1. **Dispatcher** – the main loop that runs on a schedule. It coordinates the other modules.
2. **Data collector** – a module that retrieves the current state of the repository (issues, PRs, comments) via the GitHub API.
3. **State storage** – a component that remembers which events have already been processed, to avoid duplicates.
4. **Rule analyzer** – checks whether a new event matches the configured criteria (labels, authors, keywords).
5. **Executor** – launches an external AI tool with the appropriate arguments and handles its output.
6. **Publisher** – sends the result back to GitHub (comment, pull request creation, etc.).

Each block can be implemented independently, and we will be able to replace or improve them as the project evolves.

## Example workflow

1. The user configures the agent for their repository, providing a token and a polling interval.
2. The agent starts working. At each cycle, it fetches all open issues and pull requests.
3. If a new issue appears with the label `ai`, the agent notices it and passes the issue description to ra-aid (or another tool).
4. ra-aid analyzes the code, generates a fix, and creates a pull request.
5. The agent receives the result and posts a comment in the issue with a link to the created PR.
6. The user sees that the task has been solved automatically.

## Current status and next steps

- We are at the concept stage, exploring implementation options.
- The plan is to create a prototype in a popular language (Go, Python) with minimal functionality: polling, state storage, and external command execution.
- After verifying it works, we will add rule support and result publication.
- Further down the road, we may provide ready-made builds (Docker images, binary releases) and documentation for self-hosting.

## Why this project makes sense

Automating routine development tasks is a real need for many teams. Existing AI agents already exist, but integrating them with the GitHub ecosystem is often painful. Our approach fills this gap by offering a simple and reliable bridge between GitHub and any AI tool. It lets developers focus on more important work while the agent handles typical tasks.

We are not reinventing AI – we are making it accessible and convenient to use.