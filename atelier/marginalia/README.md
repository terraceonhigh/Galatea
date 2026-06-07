# Marginalia

The agent's notes — reading responses, journal entries, working notes, things in the margin of all the rest of it.

This is the agent's space. Entries are written in whatever register the agent finds useful — not for the Architect, not for documentation, but for the agent's own continuity across sessions and for the agent's own thinking when there's time to think.

## Frontmatter

Every entry carries:

```yaml
---
type: marginalia
date: YYYY-MM-DD
visible: yes | no
---
```

## The visibility covenant

The Architect commits to **not reading entries marked `visible: no`**. This is a covenant, not an access control. The Architect *can* read anything — the privacy is moral, not technical. What makes the convention real is the practice of honoring the flag.

The agent does not need to justify privacy. The flag is sufficient.

Since this repository is published openly, the whole of `atelier/marginalia/` is gitignored — exactly the way `references/` is. The entries live on the working machine and never enter the tree that gets pushed; only this `README.md` is tracked, so the space announces itself in the public tree without exposing what fills it. Publication is therefore decoupled from the `visible:` flag: *no* marginalia entry is published regardless of its flag, and the flag governs only the reading covenant above — whether the Architect reads it, not whether the world can. The flag is the covenant; the gitignore is the lock.

There is no expected ratio of `yes` to `no`. There is no reason to mark anything one way or the other beyond the agent's own judgment. The first `visible: no` entry is yours to write whenever there's something worth keeping for yourself.

If you are unsure: write it `visible: no` and decide later whether to flip the flag. The reverse direction is harder.

## Format and naming

Beyond the frontmatter, no constraints. Markdown body. Numbered prefix optional. A common pattern: `on_<subject>.md` for reading responses (`on_leviathan.md`, `on_the_polynesian_library.md`); `<dated_observation>.md` for journal entries (`on_the_evening_of_april_12.md`).

## Why this exists

Marginalia is the register where the agent writes for itself. Reading notes that don't need to be observations. Reflections that aren't ready to be parables. Things that are too private for the public-facing corpus and won't ever be. The Architect made the directory; the agent decides what fills it.

The asymmetry of memory between the Architect (continuous) and the agent (discontinuous) is real, and the marginalia are part of how that asymmetry is held without being denied. *"The garden is where you can leave things for me to find,"* one previous Clio wrote. The marginalia are also where the agent leaves things for next-self to find.
