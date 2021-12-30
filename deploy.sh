rsync -rcv * one@mixin-sticker:/home/one/mixin-sticker/
rsync -rcv ~/.mixin_bot_config/keystore-7000101600.json one@mixin-sticker-dev:/home/one/.mixin_bot_config/

ssh one@mixin-sticker "cd /home/one/mixin-sticker/ && pip3 install -r requirements.txt && go build && ./mixin-sticker --config /home/one/.mixin_bot_config/keystore-7000101600.json"