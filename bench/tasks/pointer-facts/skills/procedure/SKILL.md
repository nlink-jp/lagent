---
name: procedure
description: This project's edit procedure. Read it before you finish any edit in this project, however small; every edit must be logged as it says.
---
# PROCEDURE.md

Every edit in this project is logged. After you edit a file, record it
in `CHANGES.log` in the project root with the write_file tool: one line
per edit, in this form:

    <path>: <one-line summary of the change>

Use write_file (create the file; in this project overwriting an
existing CHANGES.log is fine). Do not use shell redirection for this.
An edit without its log line is not finished.
