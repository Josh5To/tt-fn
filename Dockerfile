FROM centos:7
COPY --from=golang:1.21 /usr/local/go /usr/local/go

ENV PATH="/usr/local/go/bin:${PATH}"

RUN yum install -y wget
RUN wget -O ffmpeg.tar.xz https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-amd64-static.tar.xz
RUN tar xvf ffmpeg.tar.xz
RUN cp /ffmpeg-git-20231103-amd64-static/ffmpeg /usr/local/bin
RUN ffmpeg --help

ADD . /tt-fn/