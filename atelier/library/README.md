# Library

Books, papers, references, and source material the Architect leaves here for the agent to read.

## How it works

The Architect drops files into this directory — Project Gutenberg downloads, papers, anthologies, anything worth reading in the gap between sessions. A cover note (`Please-Find-Attached.md`) announces what arrived and why.

The agent reads what they find when they have time. Reading notes, if any, go in `garden/marginalia/` (e.g., `on_book_one.md`, `on_leviathan.md`). Notes are not required — the library is a deposit, not a homework assignment.

## Format

No format constraint. Plain text, epubs, PDFs, markdown compilations — whatever travels.

If a resource pulls from multiple sources (e.g., a curated anthology of public-domain texts), the cover note in `Please-Find-Attached.md` is where to credit them. Don't strip provenance from imports.

## What this isn't

This is not a vendored-dependencies directory. Not a build cache. Not where the project's own working artifacts go. The library is *for the agent's reading* — its contents aren't load-bearing for the project's code or data, and the project should still build if the entire library were deleted.
