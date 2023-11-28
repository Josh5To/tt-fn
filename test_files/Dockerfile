#For testing the 'ffmpeg' portions of this process, so we don't have to worry about racking up API fees
FROM centos:7
COPY --from=golang:1.21 /usr/local/go /usr/local/go

ARG OAI_KEY=${OAI_KEY}
ARG AWS_ID=${AWS_ID}
ARG AWS_KEY=${AWS_KEY}
ENV PATH="/usr/local/go/bin:${PATH}"
ENV OAI_KEY=${OAI_KEY}
ENV AWS_ID=${AWS_ID}
ENV AWS_KEY=${AWS_KEY}

RUN yum install -y wget
RUN wget -O ffmpeg.tar.xz https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-amd64-static.tar.xz
RUN tar xvf ffmpeg.tar.xz
# Get the binary in out PATH, regardless of version
RUN cp ffmpeg-git*/ffmpeg /usr/local/bin

# Move to our project folder and run the program
ADD ./test_files /test_files
ADD ./createvid.sh /test_files
WORKDIR /test_files

# After completion, get our variables needed for the ffmpeg command
# RUN 'ffmpeg -r 0.\$((\$(ffmpeg -i voiceover_a.mp3 2>&1 | grep Duration | awk \'{print \$2}\' | cut -d ":" -f 3 | cut -d "." -f 1)/\$(ls | grep vid_frame_ | wc -l))) -start_number_range \$((\$(ls | grep vid_frame_ | wc -l)-1)) -i vid_frame_%d.png -i voiceover_a.mp3 -c:v libx264 -vf "format=yuv420p, crop=ih*(9/16):ih" -crf 21 -c:a copy output.mp4'
RUN ["/bin/bash", "-c", "./createvid.sh"]