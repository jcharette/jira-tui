# Lessons

- When the user says the existing dirty tree is working, treat those changes as intentional working
  state. Do not frame them as suspicious or unrelated caveats; verify the whole tree and commit the
  cohesive working set if asked.
- For TUI focusable lists, keep the selected row visible even before the sub-mode is activated.
  Hiding the cursor until focus mode makes a selected section look non-interactive and can make
  users think the feature disappeared.
- If a TUI section displays a selection cursor, route movement keys to that cursor immediately.
  Do not show a cursor while leaving `j/k` or arrow keys bound to unrelated panel scrolling.
