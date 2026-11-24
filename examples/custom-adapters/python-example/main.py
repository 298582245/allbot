import os
import sys

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '../../../sdk/python')))
from allbot_adapter import run_adapter


class Adapter:
    def start(self, config, emit, context):
        if not config.get('token'):
            raise ValueError('token is required')
        self.emit = emit
        print('Callback URL:', context['callback_url'], file=sys.stderr)
        # Connect to the platform and call self.emit(unified_message) for incoming messages.

    def stop(self):
        pass

    def send_text(self, data):
        print(f"send text to {data['target']}: {data['text']}", file=sys.stderr)

    def send_image(self, data): raise NotImplementedError('Image sending is not implemented in this sample')
    def send_audio(self, data): raise NotImplementedError('Audio sending is not implemented in this sample')
    def send_video(self, data): raise NotImplementedError('Video sending is not implemented in this sample')
    def send_file(self, data): raise NotImplementedError('File sending is not implemented in this sample')
    def send_markdown(self, data): raise NotImplementedError('Markdown sending is not implemented in this sample')
    def send_rich(self, data): raise NotImplementedError('Rich messages are not implemented in this sample')
    def send_buttons(self, data): raise NotImplementedError('Buttons are not implemented in this sample')
    def send_text_with_sequence(self, data): return self.send_text(data)
    def get_user_info(self, data): return {'user_id': data['user_id'], 'nickname': '', 'avatar': '', 'extra': {}}
    def get_group_info(self, data): return {'group_id': data['group_id'], 'name': '', 'member_count': 0, 'extra': {}}
    def at_user(self, data): return self.send_text({'target': 'group_' + data['group_id'], 'text': '@' + data['user_id']})
    def delete_message(self, data): raise NotImplementedError('Message deletion is not implemented in this sample')
    def mute(self, data): raise NotImplementedError('Mute is not implemented in this sample')


run_adapter(Adapter())
