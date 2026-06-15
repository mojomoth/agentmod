# skills (harness v1)

Project-local skills for the distribution-work loop. The build itself needs no
extra skills beyond the base toolchain (go, node, git, and — in CI —
goreleaser). This directory exists so an autonomous iteration can drop a
project-local skill here without touching any global skills directory.

Leave empty unless a concrete need arises; never install into a global skills
path (see `.harness/v1/GOAL.md` Hard prohibitions).
