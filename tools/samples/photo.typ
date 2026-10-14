// Sample picture for the image tools: one subject (a mango with a leaf) on a plain
// background, so background removal has something clear to find. MIT-0.
#set page(width: 640pt, height: 480pt, margin: 0pt,
  fill: gradient.linear(rgb("#dfe7ee"), rgb("#b9c7d3"), angle: 90deg))
// The shadow on the table.
#place(dx: 200pt, dy: 372pt, ellipse(width: 250pt, height: 34pt, fill: rgb(60, 70, 80, 70)))
// The mango.
#place(dx: 190pt, dy: 120pt, ellipse(width: 260pt, height: 270pt,
  fill: gradient.radial(rgb("#ffd34d"), rgb("#f39c12"), rgb("#c0562b"), center: (35%, 30%), radius: 75%)))
// Its stalk and leaf.
#place(dx: 314pt, dy: 92pt, rect(width: 12pt, height: 38pt, radius: 4pt, fill: rgb("#6b4423")))
#place(dx: 322pt, dy: 70pt, rotate(-25deg, ellipse(width: 120pt, height: 46pt,
  fill: gradient.linear(rgb("#3d8b37"), rgb("#1f5e1c")))))
