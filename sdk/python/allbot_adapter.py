import json
import sys
import traceback


def run_adapter(adapter):
    protocol_stdout = sys.stdout
    sys.stdout = sys.stderr

    def emit(message):
        protocol_stdout.write(json.dumps({"event": "message", "message": message}, ensure_ascii=False) + "\n")
        protocol_stdout.flush()

    actions = {
        "send_text": "send_text", "send_image": "send_image", "send_audio": "send_audio",
        "send_video": "send_video", "send_file": "send_file", "send_markdown": "send_markdown",
        "send_rich": "send_rich", "send_buttons": "send_buttons",
        "send_text_sequence": "send_text_with_sequence", "get_user_info": "get_user_info",
        "get_group_info": "get_group_info", "at_user": "at_user",
        "delete_message": "delete_message", "mute": "mute",
    }

    for line in sys.stdin:
        request = None
        try:
            request = json.loads(line)
            action = request.get("action")
            data = request.get("data") or {}
            if action == "start":
                if str(data.get("protocol_version")) != "1":
                    raise ValueError("Unsupported protocol version")
                adapter.start(data.get("config") or {}, emit, {"callback_url": data.get("callback_url", "")})
                response = {"id": request.get("id"), "ok": True, "data": {}}
            elif action == "stop":
                stop = getattr(adapter, "stop", None)
                if stop:
                    stop()
                response = {"id": request.get("id"), "ok": True, "data": {}}
            elif action == "http_callback":
                callback = getattr(adapter, "handle_http_callback", None)
                result = callback(data) if callback else {"status": 404, "headers": {"Content-Type": "text/plain"}, "body": "Not found"}
                response = {"id": request.get("id"), "ok": True, "data": result or {"status": 200, "body": ""}}
            else:
                method_name = actions.get(action)
                method = getattr(adapter, method_name, None) if method_name else None
                if not method:
                    raise ValueError("Unsupported action: " + str(action))
                result = method(data)
                response = {"id": request.get("id"), "ok": True, "data": result or {}}
            protocol_stdout.write(json.dumps(response, ensure_ascii=False) + "\n")
            protocol_stdout.flush()
            if action == "stop":
                break
        except Exception as error:
            if request and request.get("id"):
                response = {"id": request["id"], "ok": False, "error": str(error)}
                protocol_stdout.write(json.dumps(response, ensure_ascii=False) + "\n")
                protocol_stdout.flush()
            else:
                traceback.print_exc(file=sys.stderr)
