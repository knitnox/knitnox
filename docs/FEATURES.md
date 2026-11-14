# Features

Knitnox is a private AI harness for personal use or a small pro-AI team: one program, one portable database, a local model or a cloud provider. Use it together in a group, in private in a chat of your own, on demand like a normal chat, or around the clock with agents.

This is everything it does today, grouped by area. Limits are listed at the end so nothing here is a surprise.

## Chat

| Feature | What it means |
|---|---|
| Streaming answers | Text appears as it is written |
| Background generation | Answers keep being written after you close the tab |
| Message queue | Up to 10 waiting messages per chat, answered in order |
| Stop, edit, regenerate, retry | Stop an answer, edit your latest message, or ask again |
| Thinking view | Models that share their reasoning show it in a collapsible block; each person can switch it off |
| Quiet tool use | Tools the AI uses are one line per answer ("Used 2 tools") that opens on a click; each person can ask for every call as its own row |
| Markdown and code | Tables, lists, quotes, syntax highlighting, a Copy button on code |
| Long-chat memory | Older messages are summarised automatically; originals are kept |
| Context meter | Shows how full the model's memory is for this chat |
| Pictures in the chat | With a picture model set up, ask the AI in your own chat for a picture and it appears in the conversation |
| Read aloud | With a voice model set up, a speaker button under every answer reads it to you; the recording is kept, so the next listen is instant |
| Never stuck | If the AI provider goes silent or hangs up, the answer stops with what it has and you can retry |
| Pictures from other sites wait | A picture hosted elsewhere shows as a button and loads only when you click it |

## Files

| Feature | What it means |
|---|---|
| Images | PNG, JPEG, WebP and GIF for models that can see |
| Documents | PDF, text, Markdown, CSV, JSON and code files; the text is read for the AI |
| Easy attaching | Button, paste or drag and drop; up to 6 files per message |

## Finding and organising

| Feature | What it means |
|---|---|
| Search and jump | `Ctrl+K` finds chats, agents and pages from anywhere |
| Full-text search | Search inside every chat you belong to |
| Pin and archive | Per person, even in shared chats |
| Rename and delete | From the sidebar or the chat menu |
| Export | Download a chat as Markdown or JSON |

## Groups

| Feature | What it means |
|---|---|
| A kind of its own | A group is started as a group. Your own chats never become one, so what your personal tools answered never reaches other people |
| Invite links | With an expiry date and a maximum number of uses |
| Roles | Admin, member (can chat) and viewer (read only) |
| One shared queue | Everyone's messages are answered one at a time |
| Its own memory | The AI saves what the group decides. Every member can read it; only the AI writes it; wrong facts can be deleted |
| Its own tools | Tool servers added for the group by its admin |
| One model | Chosen by the admin, from the shared models or the admin's own |
| No personal tools | Nobody's Memory, Plan Book, Feed, pictures, agents or tool servers are used in a group |
| Take to my chat | Under any message: a chat of your own that quotes it, can read the group, and uses your own tools |

## Read-only links

| Feature | What it means |
|---|---|
| Show a chat | Anyone who has the link can read one of your own chats, without an account |
| A copy in time | The link shows the chat as it was when you made or updated it |
| Tools hidden | Tool calls and results are left out unless you choose to show them |
| Under your control | Stop the link at any time; an admin can turn links off for the whole server |

## Models and keys

| Feature | What it means |
|---|---|
| Any OpenAI-compatible provider | OpenAI, OpenRouter, Groq, Ollama, LM Studio and others |
| Shared providers | Added by admins, available to everyone |
| Personal keys | Each user can add their own provider and key |
| Model discovery | Fetch the provider's model list and tick the ones you want |
| Custom instructions | Per user and per chat |
| Date and time aware | The AI is told your local date and time, using your device's time zone |

## Tools (MCP)

| Feature | What it means |
|---|---|
| Remote MCP servers | Give the AI tools over HTTP |
| Two modes per server | Run automatically, or ask a person first |
| Approval cards | Allow once, Deny, or Always allow, with push notification |
| Tools menu in every chat | The **Tools** button next to Attach shows what the AI may use in this chat and lets you switch off memory, the Plan Book, the Feed, pictures, connected agents or any tool server, for this chat only |
| Step limit and retries | A turn cannot loop forever. A call that could not be delivered is tried again; one that may already have run is not, so nothing is sent or booked twice |

## Agents

| Feature | What it means |
|---|---|
| Saved agents | A name, instructions, a model and chosen tools |
| Schedules | On demand, or every 15 minutes up to weekly |
| To-do list | Tasks with status and result; added by you, a chat or the agent |
| Fresh every run | An agent sees its job, tasks and memory, not old conversations |
| Tool allowlist | Allow a whole server or only named tools |
| Own MCP servers | Servers that only one agent uses |
| Connect to chats | From the home screen or the chat's Agents button |
| Results come back | A finished task is posted into the chat that asked |
| Adjustments and notes | Guidance a chat may edit; notes the agent keeps for itself |
| Read your memory | Optional, per agent, read-only |
| Use your Plan Book | Optional, per agent: read only, or add and change tasks |
| Post to your Feed | Optional, per agent, with a limit of posts per day |
| Limits | Steps per run, automatic runs per day, pause switch |
| Agent store | Share an agent with a guide; others install their own copy |

## Feed

| Feature | What it means |
|---|---|
| Short posts | One page of posts, newest first, each at most 500 characters |
| Reports | A longer text (Markdown) can sit behind "Read the full report" |
| Charts and diagrams | The AI can add a small bar or line chart from numbers, or a diagram it draws itself. Neither needs an extra model |
| Pictures, voice and video | With a picture, voice or video model set up, a post can carry an image, the text read aloud, or a short video |
| Picture check | A picture asked for by the AI is looked at once by a model that can see images, and drawn again if it does not match |
| Your models or the server's | Each user picks the model for each kind in Feed settings, or adds their own right there |
| Three kinds of author | You, the AI in your own chats, and agents you allowed |
| Private first | A post is seen only by you until you share it with everyone on the server |
| Only posts on the page | Writing a post, settings and explanations are one click away, not on the page |
| Views and filters | Latest, Mine, Shared and Bin; filter by who wrote it or by one agent; a search box |
| Read count | A shared post shows how many other people have seen it |
| Loads as you scroll | The newest 20 posts first, then 10 more each time you reach the end |
| Live without jumping | New posts are announced by a button instead of moving what you are reading |
| Agent limits | Each agent posts only if you allow it, and at most a set number of times per day |
| Full control | Change, share, unshare or delete any of your posts; deleted posts can be restored for 30 days |
| New count | The sidebar shows how many posts you have not looked at |
| Admin control | Sharing can be turned off for the whole server; an admin can take a shared post out of the public feed |

## Plan Book

| Feature | What it means |
|---|---|
| Your own tasks | A to-do list with notes, due dates and three states: to do, in progress, done |
| Projects | Group tasks that belong together; each project shows its progress |
| Views | To do, Today (due or late), Upcoming and Done, a project filter, a search box and the bin |
| Same layout as Memory | Three tabs: Tasks, Projects and Settings; one row of controls above the list |
| Quick add | One box to add a task; date, project and note open when you click in it |
| Dates without a calendar | Today, Tomorrow and Next week are one click, when adding a task and when moving one |
| Your week | Today in one sentence and the next seven days with their task counts; click a day to see only that day |
| Undo | Adding, finishing, moving and deleting a task each offer Undo |
| Catch up | Move all late tasks to today in one click |
| Ask the AI | In your own chats the AI can add, list, change and tick off tasks, once you allow it |
| Agents can help | Per agent: no access, read only, or full access |
| Who changed what | Every task keeps a history: you, the AI, or which agent |
| Bin | Deleted tasks can be restored for 30 days |
| Due count | The sidebar shows how many tasks are due today or late |
| Private | Never used in groups |

## Memory

| Feature | What it means |
|---|---|
| Long-term memory | The AI saves short facts you share and looks them up in later chats |
| Off until you turn it on | Each person switches it on for themselves under **Memory** |
| Nothing hidden | Read, search, change, pin and delete every memory; add your own |
| Things and connections | People, places and projects your memories name, and how they belong together |
| Map | Click through connected things; arrows show which way a connection reads; it loads only what you look at |
| Word search | Works with no setup |
| Smart search | With an embedding model it also finds different wording; set by the admin or by you |
| Pinned memories | A few facts the AI is always told, without searching |
| Private | Never used in groups (a group has a separate memory of its own); an agent can read it only if you allow that agent |
| Your data | Download everything as a file, or delete everything |

## Notifications and app

| Feature | What it means |
|---|---|
| Install as an app | Works as a PWA on desktop and phone |
| Push notifications | When an answer is ready or a tool needs approval |
| Phone layout | Slide-out sidebar and bottom-sheet menus |
| Light and dark theme | Follows your device, or choose one yourself |
| Accent colour | Six colours to pick from, saved per browser |
| Keyboard shortcuts | New chat, search and jump, send |
| Keyboard and screen-reader support | Skip link, visible focus, labelled controls, reduced-motion support |

## Administration

| Feature | What it means |
|---|---|
| Users | Create, rename, disable, delete, make admin, reset password |
| Sign-up | Closed unless the admin opens it; new accounts can get a token allowance |
| Quotas | Token limits per user, per day or per month |
| Usage and cost | Per user and per model |
| Audit log | Who changed what in the admin panel |
| Global settings | Sign-up, queue size, step limits, time limits, system prompt, parallel answers |
| Automatic backups | A checked copy of the database, the key and all files every day; the newest seven are kept; **Back up now** and the last result are on the settings page |
| Housekeeping | Old usage and audit rows and old agent runs are removed by limits the admin sets; chats are never removed |
| Command-line tools | List accounts, reset a password, rename a user, create an admin, make a backup |
| Health check and log file | An address that says whether the server is well; an optional log file that cannot grow without end |

## Help

| Feature | What it means |
|---|---|
| Getting started list | The home screen shows new accounts what to set up, and ticks off each step |
| Explanations in place | A "?" next to unfamiliar terms, examples that fill the agent form, hints in empty screens |
| Guides in the app | User guide, this feature list and the developer guide |
| Issues | Anyone can report a bug, idea or question; admins reply and close |

## Security

| Feature | What it means |
|---|---|
| Encrypted secrets | API keys and MCP headers are encrypted at rest |
| Login protection | Too many wrong passwords from one place lock that place out for a while, without locking out the real owner |
| Account changes | Change your username or password in Settings; a new password signs your other devices out |
| Network guard | A user's own provider or tool address cannot point at cloud credential addresses, and the admin can also close the server's own network |
| Keeps running | A fault in one answer or agent run ends only that run |
| Strict browser policy | No inline scripts or styles; model output is sanitised |
| Safe uploads | File types are checked by content, not by name |

## Known limits

- **One server process.** Knitnox runs as a single instance.
- **No e-mail.** A forgotten password is reset by an admin, not by a link.
- **Many tabs without HTTPS.** Over plain `http://`, six or more open Knitnox tabs make further pages wait. Over HTTPS there is no such limit.
- **Backups are full copies** on a folder you choose; copying them to another machine is up to you.
- **MCP over HTTP only.** No stdio servers, no OAuth sign-in flow; static headers work.
- **Files.** No OCR for scanned PDFs; Word, Excel and PowerPoint are not read.
- **Search.** Matches the beginnings of words in message text, not inside attached files.
- **Approval waits.** A tool call waiting for approval holds that chat's queue until someone decides.
- **Groups.** A group's memory has no pinning. Its things and connections can be looked at on a map but not renamed or merged: only the AI changes them. A chat cannot be turned into a group or back. When the admin changes the model, the chat does not say so. The AI of a connected chat reads a group; it cannot write to it.
- **Read-only links.** One link per chat. Recordings made with Read aloud are not part of it.
- **Plan Book.** Due dates are days, not times; there are no reminders, subtasks, repeating tasks or shared projects yet.
- **Feed.** No likes, comments or follows. One picture, recording or video per post; you cannot upload your own files to a post. Video uses one format only (`POST /videos`, then asking until it is done), which OpenRouter and DeepInfra offer; OpenAI shut its own video service down in September 2026, and Gemini's voice and video are not reachable through its OpenAI-compatible address. The read count is the number of other people who were shown a shared post.
- **Tools menu.** It switches whole groups and whole tool servers, not single tools, and it can only switch off what is already set up. Changing it in the middle of a chat makes the next reply cost a little more once.
- **Pictures and voice in chats.** Pictures are made only in your own chats, not in groups or by agents, and the AI cannot see the picture it made. Read aloud reads the first 4,000 characters of an answer and leaves code out. Videos are made only for Feed posts.
- **Model kinds.** What a model is for is guessed from its name when it is added and can be changed in the providers list. Pictures, recordings and videos are not counted in token quotas; the admin sets a number per person per day instead.
- **Memory search.** Smart search needs an embedding model; without one, only shared words match. Embedding calls are not counted in token quotas.
- **Memory size.** Built for thousands of memories per person, not millions.
- **Agents and memory.** Agents can read your memory when allowed, never write to it.
- **Agents do not talk to each other.** A chat's AI is the only manager.
- **Agent results in a chat.** Once a result is posted, the message before it can no longer be edited or regenerated.
- **Issues are simple.** One reply from an admin per issue; no comment threads or attachments.
