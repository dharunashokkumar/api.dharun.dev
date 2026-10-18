package api

func init() {
	mcpTestCalls["run_code"] = `{"language":"python","code":"print(1)"}`
	mcpTestCalls["start_sandbox_session"] = `{"image":"code"}`
	mcpTestCalls["run_in_sandbox_session"] = `{"id":"ses1","command":"ls"}`
	mcpTestCalls["stop_sandbox_session"] = `{"id":"ses1"}`
	mcpTestCalls["screenshot_web_page"] = `{"url":"https://example.com"}`
	mcpTestCalls["web_page_to_pdf"] = `{"html":"<p>hi</p>"}`
	mcpTestCalls["read_web_page"] = `{"url":"https://example.com"}`
	mcpTestCalls["create_request_bin"] = `{}`
	mcpTestCalls["read_request_bin"] = `{"id":"example","token":"example"}`
	mcpTestCalls["create_postgres_database"] = `{}`
}
