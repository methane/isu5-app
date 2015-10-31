# 簡易リグレッションテスト用の何か

## 使い方

これで、リクエストとレスポンスが `outpus.txt` に出力される。

```
bundle install
ruby test.rb > outputs.txt
GET /initialize HTTP/1.1
6.748857[s]

Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
GET / HTTP/1.1
0.069547[s]

Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
POST /login HTTP/1.1
0.079012[s]

Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
GET / HTTP/1.1
0.181738[s]

Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
GET /diary/comment/947 HTTP/1.1
0.083656[s]

Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
GET /friends HTTP/1.1
0.110723[s]

Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
POST /login HTTP/1.1
0.104519[s]

Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
Cookie#domain returns dot-less domain name now. Use Cookie#dot_domain if you need "." at the beginning.
GET /footprints HTTP/1.1
0.131189[s]

TOTAL: 7.509241[s]
```

正常系の `outputs.txt` と `diff` を取れば、簡単なテストくらいはできるでしょう。
