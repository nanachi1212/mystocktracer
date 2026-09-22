# Third-party notices

## Hermes Agent

mystocktracer can bundle the Hermes Agent Python runtime as an optional agent
adapter. Hermes Agent is not authored by mystocktracer and is not part of the
product-owned AI transport, configuration boundary, or UI.

- Project: Hermes Agent
- Upstream: https://github.com/NousResearch/hermes-agent
- Package: `hermes-agent`
- Build pin: `0.18.2` (the packaged artifact must use its
  `runtime-manifest.json` as the authoritative installed-version record)
- Copyright: Copyright (c) 2025 Nous Research
- License: MIT
- Distribution: when bundled, the Python runtime is under
  `resources/hermes-runtime`; its `LICENSE` file is copied into that directory
  by the package preparation step.

### MIT License

Copyright (c) 2025 Nous Research

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
