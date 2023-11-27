vidlength=$(ffmpeg -i voiceover_a.mp3 2>&1 | grep Duration | awk '{print $2}' | cut -d ":" -f 3 | cut -d "." -f 1)
nim=$(ls | grep vid_frame_ | wc -l)

ffmpeg -r 0.$(($vidlength/$nim)) -start_number_range $(($nim-1)) -i vid_frame_%d.png -i voiceover_a.mp3 -c:v libx264 -vf "format=yuv420p, crop=ih*(9/16):ih" -crf 21 -c:a copy output.mp4