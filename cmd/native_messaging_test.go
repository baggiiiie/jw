package cmd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeNativeMessage(t *testing.T, data any) []byte {
	t.Helper()
	payload, err := json.Marshal(data)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, binary.Write(&buf, binary.LittleEndian, uint32(len(payload))))
	buf.Write(payload)
	return buf.Bytes()
}

func decodeNativeResponse(t *testing.T, data []byte) nativeResponse {
	t.Helper()
	r := bytes.NewReader(data)

	var length uint32
	require.NoError(t, binary.Read(r, binary.LittleEndian, &length))

	payload := make([]byte, length)
	_, err := r.Read(payload)
	require.NoError(t, err)

	var resp nativeResponse
	require.NoError(t, json.Unmarshal(payload, &resp))
	return resp
}

func TestReadNativeMessage(t *testing.T) {
	input := map[string]string{"url": "https://jenkins.example.com/job/test/1"}
	encoded := encodeNativeMessage(t, input)

	msg, err := readNativeMessage(bytes.NewReader(encoded))
	require.NoError(t, err)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(msg, &decoded))
	assert.Equal(t, "https://jenkins.example.com/job/test/1", decoded["url"])
}

func TestReadNativeMessage_EmptyInput(t *testing.T) {
	_, err := readNativeMessage(bytes.NewReader(nil))
	assert.Error(t, err)
}

func TestReadNativeMessage_TooLarge(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(2*1024*1024))
	buf.Write(make([]byte, 100))

	_, err := readNativeMessage(&buf)
	assert.ErrorContains(t, err, "too large")
}

func TestWriteNativeResponse(t *testing.T) {
	var buf bytes.Buffer
	resp := nativeResponse{Success: true, Message: "Job added"}
	writeNativeResponse(&buf, resp)

	decoded := decodeNativeResponse(t, buf.Bytes())
	assert.True(t, decoded.Success)
	assert.Equal(t, "Job added", decoded.Message)
	assert.Empty(t, decoded.Error)
}

func TestWriteNativeResponse_Error(t *testing.T) {
	var buf bytes.Buffer
	resp := nativeResponse{Error: "something broke"}
	writeNativeResponse(&buf, resp)

	decoded := decodeNativeResponse(t, buf.Bytes())
	assert.False(t, decoded.Success)
	assert.Equal(t, "something broke", decoded.Error)
}

func TestHandleNativeAdd_InvalidURL(t *testing.T) {
	t.Setenv("JENKINS_USER", "user")
	t.Setenv("JENKINS_API_TOKEN", "token")

	resp := handleNativeAdd("not-a-url")
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "http://")
}

func withNoDaemon(t *testing.T) {
	t.Helper()
	orig := pidfileIsDaemonRunning
	pidfileIsDaemonRunning = func() (int, bool) { return 0, false }
	t.Cleanup(func() { pidfileIsDaemonRunning = orig })
}

func TestHandleNativeAdd_Success(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("JENKINS_USER", "user")
	t.Setenv("JENKINS_API_TOKEN", "token")
	withNoDaemon(t)

	resp := handleNativeAdd("https://jenkins.example.com/job/test/1")
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Message, "Job added")
}

func TestHandleNativeAdd_Duplicate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("JENKINS_USER", "user")
	t.Setenv("JENKINS_API_TOKEN", "token")
	withNoDaemon(t)

	url := "https://jenkins.example.com/job/test/1"
	handleNativeAdd(url)

	resp := handleNativeAdd(url)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Message, "already being monitored")
}

func TestHandleNativeAdd_NoCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("JENKINS_USER", "")
	t.Setenv("JENKINS_API_TOKEN", "")
	t.Setenv("JENKINS_TOKEN", "")

	resp := handleNativeAdd("https://jenkins.example.com/job/test/1")
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "credentials")
}
