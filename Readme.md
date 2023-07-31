# Scrubbish

Poor man's ExifTool, but it's in Go, only does stripping or copying of metadata, and only supports JPEGs.

---

Refer to the godoc for usage details. Install using `go install github.com/appgurueu/scrubbish@latest`.

---

Sparked by [an answer I gave on SO](https://stackoverflow.com/questions/76777412/copy-metadata-fom-one-jpeg-to-another-in-go/76779756#76779756).
The name can be read as either "scrub"-ish, after Rob Pike's [`scrub`](https://github.com/robpike/scrub/) utility, which has a similar purpose,
or "sc-rubbish", since this is after all (strictly?) inferior to `exiftool`.
