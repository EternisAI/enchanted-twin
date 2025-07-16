#!/usr/bin/env python3

import json
import time
from http.server import HTTPServer, BaseHTTPRequestHandler

class Model:
    def __init__(self):
        self.is_loaded = False
        self.model_name = "FakeModel-v1.0"

    def load(self):
        print(f"Loading {self.model_name}...")
        time.sleep(5)  # Simulate model loading
        self.is_loaded = True
        print(f"{self.model_name} loaded successfully!")

    def infer(self, input_text):
        if not self.is_loaded:
            raise RuntimeError("Model not loaded")

        print(f"Processing inference: {input_text}")
        time.sleep(0.1)  # Simulate inference time
        return f"Processed: {input_text}"

model = Model()

class ModelHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == '/infer':
            content_length = int(self.headers['Content-Length'])
            post_data = self.rfile.read(content_length)

            try:
                data = json.loads(post_data.decode('utf-8'))
                input_text = data.get('input', '')

                response = model.infer(input_text)

                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({"output": response}).encode('utf-8'))

            except json.JSONDecodeError:
                self.send_response(400)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({"error": "Invalid JSON"}).encode('utf-8'))
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        print(f"[{self.address_string()}] {format % args}")

def main():
    print("Starting model server...")
    model.load()

    server = HTTPServer(('localhost', 0), ModelHandler)
    port = server.server_address[1]
    print(f"SERVER_PORT:{port}")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("Shutting down...")
    finally:
        server.server_close()

if __name__ == "__main__":
    main()
