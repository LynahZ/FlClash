const fs = require('node:fs');

function stableTags(pages, suffix = '') {
  const pattern = new RegExp(`^v(\\d+)\\.(\\d+)\\.(\\d+)${suffix}$`);
  return pages.flat().flatMap(({ tag_name, draft, prerelease }) => {
    const match = pattern.exec(tag_name);
    if (draft || prerelease || !match) return [];
    return [{ tag: tag_name, version: match.slice(1).map(Number) }];
  });
}

function compare(left, right) {
  return left.version[0] - right.version[0] ||
    left.version[1] - right.version[1] ||
    left.version[2] - right.version[2];
}

function selectUpstreamRelease(upstreamPages, forkPages) {
  const upstream = stableTags(upstreamPages).sort(compare);
  if (!upstream.length) throw new Error('No stable upstream release found');
  const published = stableTags(forkPages, '-macc').sort(compare).at(-1);
  if (!published) return upstream.at(-1).tag;
  return (upstream.find(release => compare(release, published) > 0) ?? upstream.at(-1)).tag;
}

if (require.main === module) {
  const [upstream, fork] = process.argv.slice(2).map(path => JSON.parse(fs.readFileSync(path, 'utf8')));
  process.stdout.write(`${selectUpstreamRelease(upstream, fork)}\n`);
}

module.exports = { selectUpstreamRelease };
