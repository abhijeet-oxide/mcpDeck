# Petstore example

One finished tool over the public [Swagger Petstore](https://petstore3.swagger.io),
which accepts any API key. `find_pets` takes a status, calls
`findPetsByStatus`, and returns id, name, category and tags instead of the
whole response. Its test runs on every save. Lint reports no findings.

With mcpDeck running (`mcpdeck`, or `mcpdeck serve --no-open`):

```sh
mcpdeck import https://petstore3.swagger.io/api/v3/openapi.json --api petstore
echo special-key | mcpdeck secret set petstore apiKey
mcpdeck toolkit create pets --apis petstore
mcpdeck push pets --dir examples/petstore/pets
mcpdeck tool test pets --file examples/petstore/pets/find_pets.ts --input '{"status":"pending","limit":1}'
mcpdeck lint pets
mcpdeck connect claude --toolkit pets
```

`mcpdeck pull pets` writes the same files back with `mcpdeck.d.ts` and
`tsconfig.json`, so an editor or coding agent gets types for `petstore.*`.

The Petstore is a shared public demo server; its data changes and it is
sometimes down. For an offline demo with login chains, OAuth and mock
scanners, run `mcpdeck dev`.
