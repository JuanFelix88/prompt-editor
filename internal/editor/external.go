package editor

// checkExternal runs on the UI loop, so reloads never race with input handling.
// Compare content, not timestamps: atomic replacements and same-size writes
// must be detected too. The baseline changes only on load or a successful save.
func (e *Editor) checkExternal() bool {
	if e.anonymous {
		return false
	}
	previous := e.externalStatus
	disk, err := Load(e.buffer.Path)
	switch {
	case err != nil:
		e.externalStatus = "reload failed: " + err.Error()
	case disk.diskExists == e.buffer.diskExists && disk.diskText == e.buffer.diskText:
		e.externalStatus = ""
	case !disk.diskExists:
		e.externalStatus = "deleted externally; buffer kept"
	case e.buffer.Dirty:
		e.externalStatus = "changed externally; local edits kept"
	default:
		b := e.buffer
		row, col := b.Row, b.Col
		b.Lines = disk.Lines
		b.diskText, b.diskExists = disk.diskText, disk.diskExists
		b.Version++
		b.SetPosition(row+1, col+1)
		e.completions = nil
		e.service.Documents.Change(e.uri, b.Text())
		e.tokens = e.service.Semantic(e.uri)
		e.externalStatus = ""
		e.status = "reloaded from disk"
		return true
	}
	return e.externalStatus != previous
}
