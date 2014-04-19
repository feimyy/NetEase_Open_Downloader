#网易公开课视频下载脚本

##功能说明
    1,理论上支持所有网易可以下载视频的公开课，
        只需提供公开课主页URL即可自动下载所有视频
    2,支持单线程和多线程模式，并可以设定线程数
    3,支持输出log到文件,并可以设定LOG等级
    4,支持下载特定集数的视频
##安装指南


###1. 安装go 1.10+
    http://code.google.com/p/go
    
###2. 下载代码（Windows用户请在git-bash里执行）
        git clone https://github.com/feimyy/NetEase_Open_Downloader
###3. 安装libiconv和gcc,(注意:如果你的go为64位版本,那么gcc也必须是64位版本)
        wget http://ftp.gnu.org/pub/gnu/libiconv/libiconv-1.14.tar.gz
        ./configure
        make
        make install

###4. 在命令行里运行
    go get github.com/feimyy/iconv
	go get github.com/feimyy/log
	cd src/
    go build 163.go



##注意事项:
	  如果在windows下编译后，编译后的文件仍然依赖libiconv的dll文件(libiconv-2.dll)