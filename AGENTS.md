# Agent instructions

## Screenshots for visual changes

Any change that alters what a page looks like (the dashboard under `internal/dashboard/static`, its templates, CSS, or text a page renders) needs before and after screenshots. Skip this for changes with no visible effect.

1. Before editing, run the app from the base branch and capture each affected page twice: desktop width (1280px) and phone width (390px).
2. After the change, capture the same pages at the same widths, in the same state.
3. Show all four images to the user in the chat before calling the work done.
4. Add them to the PR description as a table with Before and After columns and one row each for desktop and mobile.

`gh` can't upload images to a PR. Push them to the `pr-screenshots` orphan branch under `<pr-branch>/` and embed them with `https://raw.githubusercontent.com/shouryadixitisverycool/messwa/pr-screenshots/<pr-branch>/<file>.png`. Create that branch with `git switch --orphan pr-screenshots` if it doesn't exist yet. Don't commit screenshots to the PR branch itself.

The repo is public and the dashboard shows phone numbers and names. Take screenshots with test data, or blur real numbers and names before pushing.
