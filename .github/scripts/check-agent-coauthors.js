// Fails when a PR commit credits an AI agent through a Co-authored-by trailer.
// GitHub resolves trailers into Commit.authors, so this checks exactly what
// GitHub shows as co-authors; trailers without an address are ignored by
// GitHub and are not part of this check.

// Unambiguous agent or product names. Bare model names that are also human
// first names (Claude, Gemini) are matched by address or version instead.
const agentNames = new Set([
  'aider', 'antigravity', 'chatgpt', 'claude code', 'codex', 'copilot',
  'cursor', 'deepseek', 'gemini cli', 'github copilot', 'google gemini',
  'opencode', 'openai codex', 'qwen code', 'qwen-coder',
]);

// Model names are a family followed by a variant or version, e.g.
// "Claude Opus 4.5", "GPT-5" or "Gemini 3 Pro". Requiring a digit keeps human
// names such as "Claude Shannon" out.
const modelFamilies = ['claude', 'deepseek', 'gemini', 'gpt', 'qwen'];

const agentEmails = new Set([
  'noreply@anthropic.com',
  'noreply@openai.com',
  'cursoragent@cursor.com',
  'noreply@cursor.com',
  'noreply@cursor.sh',
  'copilot@github.com',
  'aider@aider.chat',
  'amp@ampcode.com',
  'qwen-coder@alibabacloud.com',
  'noreply@jules.google',
]);

// GitHub accounts that agents commit through.
const agentLogins = new Set([
  'copilot',
  'cursoragent',
  'google-labs-jules[bot]',
  'devin-ai-integration[bot]',
]);

function isAgent(author) {
  // A co-author resolved to a GitHub account is a person unless the account
  // is a known agent; the name and address heuristics are only for
  // unresolved addresses such as noreply@anthropic.com.
  if (author.user) {
    return agentLogins.has(author.user.login.toLowerCase());
  }

  const name = (author.name ?? '').trim().toLowerCase();
  const email = (author.email ?? '').trim().toLowerCase();

  const isModelName = /\d/.test(name) && modelFamilies.some(family =>
    name.startsWith(`${family} `) || name.startsWith(`${family}-`)
  );

  return agentNames.has(name) || isModelName || agentEmails.has(email);
}

async function listCommits({ github, context }) {
  const commits = [];
  let cursor = null;
  do {
    const { repository } = await github.graphql(`
      query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
        repository(owner: $owner, name: $repo) {
          pullRequest(number: $number) {
            commits(first: 100, after: $cursor) {
              nodes {
                commit {
                  id
                  oid
                  authors(first: 100) {
                    nodes { name email user { login } }
                    pageInfo { hasNextPage endCursor }
                  }
                }
              }
              pageInfo { hasNextPage endCursor }
            }
          }
        }
      }
    `, {
      ...context.repo,
      number: context.payload.pull_request.number,
      cursor,
    });
    const page = repository.pullRequest.commits;
    commits.push(...page.nodes.map(node => node.commit));
    cursor = page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null;
  } while (cursor);
  return commits;
}

async function listCoauthors({ github }, commit) {
  // GitHub lists the primary Git author first; only check co-authors.
  const coauthors = commit.authors.nodes.slice(1);
  let pageInfo = commit.authors.pageInfo;
  while (pageInfo.hasNextPage) {
    const { node } = await github.graphql(`
      query($id: ID!, $cursor: String!) {
        node(id: $id) {
          ... on Commit {
            authors(first: 100, after: $cursor) {
              nodes { name email user { login } }
              pageInfo { hasNextPage endCursor }
            }
          }
        }
      }
    `, { id: commit.id, cursor: pageInfo.endCursor });
    coauthors.push(...node.authors.nodes);
    pageInfo = node.authors.pageInfo;
  }
  return coauthors;
}

module.exports = async ({ github, context, core }) => {
  const commits = await listCommits({ github, context });

  if (commits.length !== context.payload.pull_request.commits) {
    core.error('Could not check every PR commit for agent co-authorship. Reduce the PR size or rerun the check if the PR changed.');
    return;
  }

  let invalid = false;
  for (const commit of commits) {
    const coauthors = await listCoauthors({ github }, commit);
    if (coauthors.some(isAgent)) {
      core.error(`Commit ${commit.oid} credits an AI agent as a co-author.`);
      invalid = true;
    }
  }

  if (invalid) {
    core.setFailed('Contributors, not LLMs, are responsible for commits. Remove agent Co-Authored-By trailers. Credit agents in the PR body or use an Assisted-By trailer instead.');
    return;
  }
  core.info(`Checked ${commits.length} commit(s); no agent co-authors found.`);
};
