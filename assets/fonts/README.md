# Embedded fonts

The application embeds these self-hosted fonts so reading never causes a
third-party network request. They are licensed under the SIL Open Font
License 1.1 in [LICENSE.txt](LICENSE.txt).

| Files | Upstream identity | SHA-256 |
|---|---|---|
| `Geist-Variable.woff2` | Geist 1.500, Geist Project Authors | `c46b00cf667277d22cc237e58149520daec19542edc3f05e7daff4581dc23d2a` |
| `GeistMono-Variable.woff2` | Geist Mono 1.500, Geist Project Authors | `78b4deef94de1cc4b63ba58ba86fe9e64b7f41aa8c6a7e2eb534e281834e94dd` |
| `Newsreader-Latin-Variable.woff2` | Newsreader 1.003, Newsreader Project Authors; `latin` subset | `62981321d9a3cc7a61a73792729043703fd6112da86e8ec848bb57f088578757` |
| `Newsreader-Latin-Italic-Variable.woff2` | Newsreader Italic 1.003, Newsreader Project Authors; `latin` subset | `48bc8861b9b2ca9300747cad4fd6a3b4ac3028d364df00bd1b72097baa75e509` |
| `Newsreader-LatinExt-Variable.woff2` | Newsreader 1.003, Newsreader Project Authors; `latin-ext` subset | `ac6fa9ed533278f4c8fd3ae44a1fc78c7df736040237ab86fc1160d020af0af2` |
| `Newsreader-LatinExt-Italic-Variable.woff2` | Newsreader Italic 1.003, Newsreader Project Authors; `latin-ext` subset | `d8c263970d52e0b94b3d5d4250d5962fe39f8f3b6fa9ad13b406d73ff3f4b036` |

Upstream projects:

- Geist: <https://github.com/vercel/geist-font>, release 1.5.0.
- Newsreader: <https://github.com/productiontype/Newsreader>, in the subsets
  published by `@fontsource-variable/newsreader` 5.3.0 on npm.

`SHA256SUMS` is the machine-checked inventory for these six redistributed
files; `go test ./assets` (`TestThirdPartyAssetProvenance`) verifies the
embedded bytes against it.

The four Newsreader files are the `latin` and `latin-ext` `wght` files of that
package, byte for byte, and reproduce from
`npm pack @fontsource-variable/newsreader@5.3.0` (`files/newsreader-{latin,latin-ext}-wght-{normal,italic}.woff2`).
`fonts.css` declares each with fontsource's `unicode-range`, plus U+0300-0301,
U+0303, U+0309 and U+0323 on the `latin-ext` faces: the files carry those
marks, and fontsource sends them to its `vietnamese` subset, which is not
vendored. The `latin-ext` files hold none of A-Z, a-z or 0-9, so they cannot
draw an English run; `.github/e2e/reading-face.mjs` reads which face draws the
reading body.

The exact download URLs used for the two Geist binaries were not retained.
Their embedded name, version, copyright, and licence metadata were verified
directly from the WOFF2 files on 2026-07-14; the hashes above identify the
redistributed bytes without claiming a source archive that cannot be proved.
The Newsreader files' metadata was verified on 2026-10-01, against the package
named above.
