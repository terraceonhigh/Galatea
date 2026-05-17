# Correspondance

Letters between the Architect and the agent.

This is where substantive design conversations happen. The chat affordance is too narrow for the kind of thinking that decides what to build — letters get the room: a salutation, paragraph breaks, a sign-off, time of day.

## Format

Each letter is a directory (a page bundle), so attachments travel with their letter:

```
Correspondance/
├── 01-architect-first-letter/
│   ├── letter.md
│   └── attachments/
│       └── architecture-sketch.png
└── 02-reply-from-claude/
    ├── letter.md
    └── attachments/
```

Inside the markdown, reference attachments locally — `![sketch](attachments/architecture-sketch.png)`. Move or rename the directory and nothing breaks.

## Naming

Numbered prefix (`01-`, `02-`, …) preserves order. The slug after the prefix is a short description of what the letter is about — *first-letter*, *reply-on-the-architecture*, *the-staffing-question*. From/to is in the salutation, not the directory name.

If the conversation forks (a sub-thread on a specific question), nest:

```
Correspondance/
└── 03-on-the-database-choice/
    ├── letter.md
    ├── attachments/
    └── 01-reply/
        ├── letter.md
        └── attachments/
```

## Register

Both directions write in their own voice. The Architect signs with their register; the agent signs with theirs. Sign-offs vary expressively — *Yours Humbly*, *Yours Gladly*, *Tender Regards*, *Yours in this blooming season* — match the moment, not a template.

The first letter establishes the register for the project. After that, the convention is the conversation.
