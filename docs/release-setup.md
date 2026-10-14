# Release automation: one-off setup

These are the manual steps behind the automated release process. They are
repository and organisation settings, not code, so they need an admin of
`jumppad-labs`. Each step says which pull request has to be merged first. A
check can only be made required after it has run at least once, so the order
matters.

Commands use the [GitHub CLI](https://cli.github.com/), logged in as an
admin. Everything can also be done in the GitHub web UI.

| # | Step | Repo | Do it after |
|---|------|------|-------------|
| 1 | [Release labels and the label rule](#1-release-labels-and-the-label-rule) | xcl, xcl-python | the release-label check is merged in both repos |
| 2 | [Require the bindings and test gates](#2-require-the-bindings-and-test-gates) | xcl, xcl-python | the reusable Python suite and xcl's bindings check are merged |
| 3 | [Create the release bot](#3-create-the-release-bot) | org | any time before auto-tag is merged |
| 4 | [Push the xcl-python seed tag](#4-push-the-xcl-python-seed-tag) | xcl-python | before "Python releases from its own tags" is merged |
| 5 | [Allow auto-merge on xcl-python](#5-allow-auto-merge-on-xcl-python) | xcl-python | step 2 |

## 1. Release labels and the label rule

Every pull request must carry exactly one release label. Create the four labels
in both repositories:

```sh
for repo in jumppad-labs/xcl jumppad-labs/xcl-python; do
  gh label create major      --repo "$repo" --color B60205 --description "Release: breaking change (1.0.0 while below 1.0)"
  gh label create minor      --repo "$repo" --color FBCA04 --description "Release: new feature, or a breaking change before 1.0"
  gh label create patch      --repo "$repo" --color 0E8A16 --description "Release: bug fix"
  gh label create no-release --repo "$repo" --color C5DEF5 --description "Merge without releasing (maintenance, docs)"
done
```

Then protect `main` in each repository: **Settings → Branches → Add branch
protection rule** (or **Rules → Rulesets**) for `main`:

- Require a pull request before merging.
- Require status checks to pass before merging, and add:
  - xcl: `release-label / release-label`, plus the checks that already run on
    PRs (`build`, `Build on minimum supported Go`, `Build and smoke-test libxcl`,
    `Test xcl-core on Windows`, and each `Examples` job).
  - xcl-python: `release-label / release-label` and `Check the package`.

The check names only appear in the picker after the check has run once on a PR.

**Check:** open a throwaway PR with no label; it can't be merged. Add
`no-release` and it can.

## 2. Require the bindings and test gates

On **xcl**, create the override label. Only people with triage access or above
can apply labels, so only maintainers can use it:

```sh
gh label create breaks-bindings --repo jumppad-labs/xcl --color D93F0B \
  --description "Merge although a language binding fails; fix it in the binding's engine update PR"
```

Add the `bindings` check to xcl's required checks for `main`.

On **xcl-python**, add `test / result` to the required checks for `main`.

Require only these gate checks, never the individual matrix jobs. The gates
always run and summarise everything before them.

**Check:** on xcl, a draft PR that breaks Python can't be merged; it becomes
mergeable once `breaks-bindings` is added. On xcl-python, a PR with a failing
test can't be merged.

## 3. Create the release bot

The release bot is a GitHub App that the workflows act as. Events caused by
the built-in `GITHUB_TOKEN` don't start other workflows, so without the bot
an automatic tag wouldn't start the release, and the engine update PR in
xcl-python would never run its checks or release when merged.

1. Go to **jumppad-labs → Settings → Developer settings → GitHub Apps → New
   GitHub App**.
2. Fill in:
   - **Name:** `xcl release bot`
   - **Homepage URL:** `https://github.com/jumppad-labs/xcl`
   - **Webhook:** untick **Active**
   - **Repository permissions:**
     - Contents: **Read and write** (push tags and branches)
     - Pull requests: **Read and write** (open the engine update PR, enable auto-merge)
     - Workflows: **Read and write** (push branches in repos that hold workflows)
     - Metadata: **Read-only** (set automatically)
   - **Where can this GitHub App be installed?** Only on this account.
3. **Create GitHub App**, then note the **App ID** shown at the top of the
   App's page.
4. Under **Private keys**, **Generate a private key**. A `.pem` file downloads.
5. **Install App** → jumppad-labs → **Only select repositories** →
   `xcl` and `xcl-python`.
6. Store the ID and key as secrets in both repositories:

   ```sh
   for repo in jumppad-labs/xcl jumppad-labs/xcl-python; do
     gh secret set RELEASE_APP_ID          --repo "$repo" --body "<App ID>"
     gh secret set RELEASE_APP_PRIVATE_KEY --repo "$repo" < path/to/xcl-release-bot.private-key.pem
   done
   ```

7. Delete the downloaded `.pem` file once both secrets are set.
8. If a ruleset restricts who can create tags or push to branches, add the
   `xcl release bot` App to its bypass list.

**Check:**

```sh
gh secret list --repo jumppad-labs/xcl
gh secret list --repo jumppad-labs/xcl-python
```

Both list `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY`, and the App's
installation page shows both repositories.

## 4. Push the xcl-python seed tag

xcl-python has no tags yet, but PyPI already holds 0.5.3, 0.6.0 and 0.7.0.
Tagging the commit that built 0.7.0 makes the first automatic release 0.7.1 or
higher.

1. Find the xcl-python release run for xcl `v0.7.0` under **xcl-python →
   Actions → Release** and note the commit it checked out. It is expected to
   be `a7f4993` or `d39b4de`.
2. Push the tag:

   ```sh
   cd xcl-python
   git tag v0.7.0 <commit>
   git push origin v0.7.0
   ```

Do this **before** the PR "Python releases from its own tags" is merged. At
this point the Python release still listens only to xcl's dispatch, so this
tag starts nothing. Afterwards it would rebuild 0.7.0.

**Check:** `git ls-remote --tags origin` in xcl-python shows only `v0.7.0`,
and no Release run started.

## 5. Allow auto-merge on xcl-python

Engine update PRs merge themselves once their required checks pass:

```sh
gh api -X PATCH repos/jumppad-labs/xcl-python -F allow_auto_merge=true
```

Do this after step 2, so auto-merge has the `test / result` check to wait for.

**Check:** `gh api repos/jumppad-labs/xcl-python --jq .allow_auto_merge`
prints `true`.
