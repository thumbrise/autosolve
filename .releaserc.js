/*
 * Copyright 2026 thumbrise
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const fs = require('fs');
const path = require('path');

const commitPartial = fs.readFileSync(
    path.join(__dirname, 'release-template.hbs'),
    'utf8'
);

module.exports = {
    branches: ['main'],
    plugins: [
        [
            '@semantic-release/commit-analyzer',
            {
                preset: 'conventionalcommits',
            }
        ],
        [
            '@semantic-release/release-notes-generator',
            {
                preset: 'conventionalcommits',
                presetConfig: {
                    types: [
                        {type: 'feat', section: 'Features'},
                        {type: 'fix', section: 'Bug Fixes'},
                        {type: 'ci', section: 'CI/CD'},
                        {type: 'test', section: 'Tests'},
                        {type: 'revert', section: 'Reverts'},
                        {type: 'build', section: 'Build System'},
                        {type: 'refactor', section: 'Code Refactoring'},
                        {type: 'style', section: 'Code Refactoring'},
                        {type: 'perf', section: 'Performance Improvements'},
                        {type: 'docs', section: 'Documentation'},
                        {type: 'chore', section: 'Internal Changes'},
                    ],
                },
                parserOpts: {
                    noteKeywords: [
                        'BREAKING CHANGE',
                        'BREAKING CHANGES',
                        'BREAKING',
                        '!'
                    ],
                },
                writerOpts: {
                    commitPartial,
                    commitsSort: ['scope', 'subject'],
                    includeDetails: true,
                    showBody: true,
                    bodyWrap: 100
                }
            }
        ],
        '@semantic-release/github'
    ]
};
