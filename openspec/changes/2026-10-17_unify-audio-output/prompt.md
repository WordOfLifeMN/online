# Goal
Prettier output from `audio` command

# Requirements

The current output presents all the correct data in a format like the following

```
Message #1
Video file: 2026-10-04-pm Pay Attention.mp4
Speaker   : Pastor Mary Peltz

UPLOAD PACKET
Channel : WOL
Privacy : public
Thumb   : C:\Users\WordofLifeMNMedia\Documents\WOLMessages\2026-q4 Prayer\2026-10-04-pm Pay Attention.jpg

╭───────────────────────────────────────────────────────────────────────────────────┄┄
│ Title
Pay Attention | Mary Peltz | October 4, 2026
│ Alternative
Pay Attention to God's Breadcrumbs | Mary Peltz | October 4, 2026
╰───────────────────────────────────────────────────────────────────────────────────┄┄
╭───────────────────────────────────────────────────────────────────────────────────┄┄
│ Description
Pastor Mary talks about the importance of paying attention to spiritual breadcrumbs God is laying out for us this week. She shares stories about street witnessing and how following small cues led to powerful divine connections. The key takeaway is that revival is happening right now if we stay alert and engaged with what God is doing around us.
╰───────────────────────────────────────────────────────────────────────────────────┄┄
2026/10/17 13:21:30 Timeline: Extract = 24.2677213s, Transcribe = 2m7.9383335s, Summarize = 3.5943015s
```

I would like to see the following changes:

## "box.go" utilities

Please create a `util/box` package that manages drawing the output boxes with ASCII or Exteneded ASCII characters:
- header like `╭────────...`
- section like `|────────...` (with horizontal line connected to the vertical line)
- footer like `╰──────────...`
- section title - not sure what this looks like by something like the "section" above with title string?
- text line like `| ` + a message or string

## each message in it's own "box" output 

Here's a rough example. Note:
- I'm using ascii for easy typing, I'd prefer box characters over pipes
- The title and description lines still have no leading `|`, that is on purpose to make it easier to copy whole lines
- The open right edge would be nice to close around character 78, with wrapping, except on the lines that have no opening markup (i.e. lines for copying)

```
<header>
| Message #1
| Video file: 2026-10-04-pm Pay Attention.mp4
| Speaker   : Pastor Mary Peltz
<section title: UPLOAD PACKET>
| Channel : WOL
| Privacy : public
| Thumb   : C:\Users\WordofLifeMNMedia\Documents\WOLMessages\2026-q4 Prayer\2026-10-04-pm Pay Attention.jpg
<section title: Titles>
│ Title
Pay Attention | Mary Peltz | October 4, 2026
│ Alternative Title
Pay Attention to God's Breadcrumbs | Mary Peltz | October 4, 2026
<section title: Description>
| <description>
<section title: Timeline>
| <timelines, etc>
<footer>
```

# Out of scope

- code changes unrelated to restructuring this output
  - Moving some information (like Timeline) out of logging into the output is acceptable

# Testing

- create unit tests
- Include steps in the plan to prompt me to test manually and report problems or successes