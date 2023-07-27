mkdir test

for i in test/lorem-{1..9}.md; do
  curl 'https://jaspervdj.be/lorem-markdownum/markdown.txt?num-blocks=10' > $i
done

for i in test/folder{1..5}; do
  mkdir -p $i

  for j in $i/lorem-{1..9}.md; do
    curl 'https://jaspervdj.be/lorem-markdownum/markdown.txt?num-blocks=10' > $j
  done
done
