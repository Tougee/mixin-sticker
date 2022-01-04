rm spider.log mixinsticker
rsync -rcv * root@touge-ubuntu:/home/touge/mixin-sticker/
rsync -rcv ~/.mixin_bot_config/keystore-7000101600.json root@touge-ubuntu:/home/touge/.mixin_bot_config/

ssh root@touge-ubuntu "cd /home/touge/mixin-sticker/ && pip3 install -r requirement.txt && export PATH=$PATH:/usr/local/go/bin && go build && service mixinsticker restart"