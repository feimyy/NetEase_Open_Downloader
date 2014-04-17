网易公开课视频下载脚本


安装:

go get github.com/feimyy/iconv

如果出现错误提示,请下载并安装libiconv和gcc

wget http://ftp.gnu.org/pub/gnu/libiconv/libiconv-1.14.tar.gz
./configure
make
make install

go get 成功后
直接go build 即可

注意:
    如果在windows下编译后，编译后的文件仍然依赖libiconv的dll文件(libiconv-2.dll)