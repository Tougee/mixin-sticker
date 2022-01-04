# mixin-sticker

## mixin sticker bot

Searching for `7000101600` in [Mixin Messenger](https://mixin.one/messenger) and start using.


## spider
```shell
python3 spider.py
usage: spider.py [-h] [--url URL] [--album ALBUM] [--crawl {tg,wechat}]

optional arguments:
  -h, --help           show this help message and exit
  --url URL            lottie json url, e.g., https://assets9.lottiefiles.com/packages/lf20_muiaursk.json
  --album ALBUM        Telegram sticker album name, e.g., stpcts
  --crawl {tg,wechat}  support 2 options:
                       [tg] for spider Telegram stickers from https://tlgrm.eu
                       [wechat] for spider WeChat stickers from https://sticker.weixin.qq.com/cgi-bin/mmemoticon-bin/emoticonview?oper=billboard&t=rank
```