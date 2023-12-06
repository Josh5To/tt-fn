vidlength=$(ffmpeg -i voiceover_a.mp3 2>&1 | grep Duration | awk '{print $2}' | cut -d ":" -f 3 | cut -d "." -f 1)
nim=$(ls | grep vid_frame_ | wc -l)

spf=$(( $(($vidlength/$nim)) * 60 ))
fps=$( awk -v spf="$spf" 'BEGIN { printf "%.2f", (60/spf) }' )


ffmpeg -r $fps -start_number_range $(($nim-1)) \
-i vid_frame_%d.png -i voiceover_a.mp3 -c:a aac -ac 2 -b:a 128k \
-c:v libx264 -profile:v baseline  -pix_fmt yuv420p -vf "crop=ih*(9/16):ih" \
-movflags faststart -g 1 output.mp4