# Pull request merges

All pull requests must be squash-merged into the remote `main` branch. Do not use merge commits or rebase merges.

After merge approval, use `gh pr merge --squash`, verify the resulting commit on remote `main`, and then fast-forward the local `main` branch.

Do not rewrite existing remote history without explicit authorization.
