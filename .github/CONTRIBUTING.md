# Pull request merges

All pull requests must be merged into the remote `main` branch with a merge commit, preserving the development branch's individual commits. Do not use squash merges or rebase merges.

After merge approval, use `gh pr merge --merge`, verify the merge commit and development commits on remote `main`, and then fast-forward the local `main` branch.

Do not rewrite existing remote history without explicit authorization.
