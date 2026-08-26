# Embedded sensitive-word data

## Chinese

`fwwdn_sensitive_words.txt` is a normalized snapshot derived from
[`fwwdn/sensitive-stop-words`](https://github.com/fwwdn/sensitive-stop-words)
at revision `a7d06bb1c321e669943b6841570d9da6dad8ce2b` (Apache-2.0).

The snapshot combines these upstream files, trims whitespace, and removes
case-insensitive duplicates while preserving the first occurrence:

- `色情类.txt`
- `政治类.txt`
- `广告.txt`
- `涉枪涉爆违法信息关键词.txt`

The embedded snapshot contains 1,153 entries. At startup, six ambiguous
two-character ASCII entries (`QQ`, `3P`, `LY`, `JS`, `BT`, and `SM`) are
excluded from the active default list because the matcher uses case-insensitive
substring matching. Without that guard, ordinary text such as `JSON`, `apply`,
or `small` would be blocked. The active default therefore contains 1,147
entries.

## English

`google_profanity_en.txt` is an exact snapshot of `data/en.txt` from
[`coffee-and-fun/google-profanity-words`](https://github.com/coffee-and-fun/google-profanity-words)
at revision `0ae3460863120bc671361b9403cc65d5f2075b89` (MIT). The upstream file
contains 962 non-empty lines and 950 case-insensitive unique entries. Duplicate
lines are removed at startup, and the ambiguous two-character ASCII entries
`ho` and `xx` are retained in the source snapshot but excluded from active
defaults. The active English dictionary therefore contains 948 entries.

Bundled English entries use case-insensitive ASCII word boundaries. This keeps
standalone terms detectable while avoiding substring matches such as `ass` in
`assistant`, `anal` in `analysis`, or `sex` in `sextant`. Phrases and terms
next to Chinese characters are still detected. The Chinese and English active
sets share one entry (`fuck`), leaving 2,094 unique active defaults in total.

An administrator-supplied `SensitiveWords` option remains a complete,
unfiltered override. Saving an empty value explicitly disables the list and
persists that choice across restarts. Terms that also occur in the bundled
English dictionary retain English word-boundary matching; all other custom
terms keep the existing substring behavior.
