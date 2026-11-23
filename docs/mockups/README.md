# Mockups

Source for the approved screens in `../screenshots/`. Each `.dc.html` file is a
self-contained design: inline styles hold the exact spacing and colours, and
`renderVals()` at the bottom holds the sample data.

Re-render a screenshot after changing a mockup (needs Node and Playwright with Chromium):

```sh
node docs/mockups/render.js docs/mockups/Tool.dc.html docs/screenshots/tool-builder.png
```
