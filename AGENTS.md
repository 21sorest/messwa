# Agent instructions

## Screenshots for visual changes

Any change that alters what a page looks like (the dashboard under `internal/dashboard/static`, its templates, CSS, or text a page renders) needs before and after screenshots. Skip this for changes with no visible effect.

1. Before editing, run the app from the base branch and capture each affected page twice: desktop width (1280px) and phone width (390px).
2. After the change, capture the same pages at the same widths, in the same state.
3. Show all four images to the user in the chat before calling the work done.
4. Add them to the PR description as a table with Before and After columns and one row each for desktop and mobile.

Upload them with `gh`'s `--attach` flag (gh 2.99 or later) on `gh pr create`, `gh pr edit` or `gh pr comment`. Write the table in a body file that references the local paths, such as `![Dashboard after, mobile](./after-mobile.png)`, then pass each file with `--attach`. `gh` uploads them and rewrites those references to the hosted URLs. `gh` resolves paths in a body file relative to that file's directory, so keep `body.md` next to the images. Keep both outside the repo and never commit them.

```sh
gh pr create --body-file body.md \
  --attach ./before-desktop.png --attach ./after-desktop.png \
  --attach ./before-mobile.png --attach ./after-mobile.png
```

The repo is public and the dashboard shows phone numbers and names. Take screenshots with test data, or blur real numbers and names before pushing.
