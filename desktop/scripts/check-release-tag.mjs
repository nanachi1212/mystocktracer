import { version } from './release-tools.mjs';
const tag = process.argv[2] || process.env.GITHUB_REF_NAME;
if(tag !== 'v'+version) throw new Error('Release tag must match desktop version v'+version);
console.log('Verified release tag '+tag);