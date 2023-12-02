
FROM golang:1.21

ENV CGO_ENABLED=0
# Move to our project folder and run the program
ADD . /tt-fn/
WORKDIR /tt-fn
RUN touch scenemaker
# RUN go build -o scenemaker

FROM centos:7
COPY --from=0 /tt-fn/scenemaker /app/scenemaker
COPY --from=0 /tt-fn/createvid.sh /app/createvid.sh

ENV PATH="/usr/local/go/bin:${PATH}"

RUN yum install -y wget
RUN wget -O ffmpeg.tar.xz https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-amd64-static.tar.xz
RUN tar xvf ffmpeg.tar.xz
# Get the binary in out PATH, regardless of version
RUN cp ffmpeg-git*/ffmpeg /usr/local/bin

# Move to our /app directory, deviate from main image in moving over pre-generated data (mock data from normal program run).
WORKDIR /app
COPY test_files/vid_frame* /app/
COPY test_files/voiceover_* /app/