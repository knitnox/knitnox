# User guide

Knitnox is a private AI chat for you and your team. This guide shows how to use it, one task at a time. Every section stands on its own, so jump to the one you need.

> **In a hurry?** Read "Start in three steps", then come back when you want more.

## Start in three steps

1. **Sign in.** Open Knitnox in your browser and log in with the account your admin gave you. If the login page offers **Create an account**, you can make one yourself; you type the password twice.
2. **Type a message** in the box on the home screen and press **Enter**.
3. **Read the answer.** It appears word by word. You can close the tab; the answer keeps being written and is there when you return.

If the home screen says "No models are available yet", ask your admin to add one, or add your own API key under **Settings → Your API keys**.

New accounts see a **Getting started** list on the home screen. It ticks off each step as you do it. Click **Hide** when you no longer need it.

## Find your way around

| Where | What is there |
|---|---|
| **Sidebar, top** | **New chat** and **Search or jump to…** (`Ctrl+K`) |
| **Sidebar, middle** | **Agents**, **Feed**, **Plan Book**, **Memory**, **Settings**, **Help & guides**, then your agents and your chats |
| **Sidebar, bottom** | Your name: switch between light and dark, open Settings, report an issue, log out |
| **A small "?"** | Click it for a plain-words explanation of the thing next to it |

To change how Knitnox looks, open **Settings → Appearance**: choose **Auto**, **Light** or **Dark**, and pick an accent colour. The choice is saved in the browser you are using.

## Chat basics

| I want to… | Do this |
|---|---|
| Start a new chat | Click **New chat** in the sidebar, or press `Ctrl+Shift+O` |
| Choose the AI model | Use the model picker under the message box on the home screen. Change it later under **Share → Chat settings** |
| Write several lines | Press `Shift+Enter` for a new line. `Enter` sends |
| Send while the AI is still answering | Just send. Your message waits in a queue (up to 10) and is answered in order |
| Stop an answer | Click the square **Stop** button. What was written so far is kept |
| Fix my last message | Hover it and click the pencil. The old answer is replaced |
| Get a different answer | Click the circular arrow under the latest answer |
| Copy an answer or a code block | Click the copy icon under the message, or **Copy** on the code block |
| Get a picture | Ask for one in your own chat: "paint a red bicycle by the sea". It appears in the conversation; click it to see it large. Needs a picture model (see below) |
| Have an answer read to me | Click the speaker button under the answer. The first time takes a moment; after that it plays at once. Needs a voice model (see below) |
| See a picture from another website | Click the **Show picture from …** button. Pictures from other sites are not loaded by themselves, so an answer cannot quietly send anything to another site |

On a phone, **Enter** adds a new line and the arrow button sends.

**Pictures and voice need a model.** If your admin shares a picture or voice model, both work for you with nothing to set up. If you added your own model of that kind, it is used when the admin shares none. To choose between them, or to turn one off, open **Feed → Settings → Pictures, voice and video**. If the speaker button is missing, or the AI says it cannot make pictures, no such model exists yet. Pictures are made in your own chats only, not in group chats. With the server's models there is a daily limit, shared with the Feed.

## Attach images and files

Click **Attach**, paste from the clipboard, or drag files onto the page. You can attach up to 6 files per message.

- **Images** (PNG, JPEG, WebP, GIF) need a model that can see images. If the model can't, the app tells you.
- **Documents** work with every model: PDF, text, Markdown, CSV, JSON and code files. The text inside is read and given to the AI.

Scanned PDFs without a text layer, and Word, Excel or PowerPoint files, are not read.

## Find and organise chats

- **Find anything:** press `Ctrl+K`, or click **Search or jump to…** in the sidebar. Type a few letters to find a chat, an agent or a page. Move with the arrow keys and press **Enter** to open.
- **Search inside messages:** type your words in the same box and choose **Search inside messages** at the bottom of the list.
- **Pin, archive, rename, export or delete:** click the **⋯** next to a chat.
- **Long chats:** click **Load earlier messages** at the top.

When a chat gets very long, older messages are summarised so the AI can keep going. Nothing is deleted; you can still scroll back and read everything.

## Tell the AI how to behave

- **In all your own chats:** **Settings → Custom instructions**. Example: "Answer in Bengali" or "Keep answers short".
- **In one chat only:** open the chat, click **Share**, and fill in **Instructions for this chat**.

## Chat with other people

1. Open a chat you own and click **Share**.
2. Choose **Can chat** or **Read only**, then click **Create invite link**.
3. Send the link. Anyone who opens it and signs in joins the chat.

Everyone's messages go into one queue and the AI answers them one at a time. The AI sees who wrote each message.

> **Safe by default.** Group members cannot use your tools or your personal API key unless you tick them under **Share → Chat settings**.

## Give the AI tools (MCP servers)

Tools let the AI do things outside the chat, such as search your mail or read a calendar. Tools come from **MCP servers**.

1. Go to **Settings → Tools → Add an MCP server**.
2. Enter a name and the server's URL. Add headers if the server needs a key.
3. Choose what happens when the AI wants to use a tool:
   - **Run it automatically**: fine for tools that only read.
   - **Ask me before running**: best for tools that change things. The chat shows a card with **Allow once**, **Deny** and **Always allow**.
4. Click **Test** to see the server's tools.

Your own chats use all of your enabled servers, unless you switch one off for a chat (next section).

## Choose the tools for one chat

Click **Tools** next to **Attach**, on the home screen before you start or in any chat you own.

| Row | What it switches |
|---|---|
| **Memory** | Saving facts about you and looking them up |
| **Plan Book** | Adding, listing and finishing your tasks |
| **Feed** | Posting to your feed and reading your posts |
| **Pictures** | Making a picture in the chat |
| **Agents** | Handing work to the agents connected to this chat |
| Each tool server | Every tool of that server |

- A switch changes **this chat only**. Other chats, and the setting itself, stay as they are.
- A greyed-out row is something you have not turned on yet; **Open** takes you to where that is done.
- The button shows how many things are off, for example **Tools · 2 off**.
- Fewer tools make answers cheaper and keep the AI on the point. Switching Memory off is also the way to have one chat the AI remembers nothing from.
- **In the middle of a chat** the menu shows a note: after a change, the next reply is a little slower and costs a little more, once, because the AI provider has to read the conversation again from the start. Choosing before the first message costs nothing.
- In a group chat only the owner sees the button, and the personal rows (Memory, Plan Book, Feed, Pictures) are off for everyone.

## Agents: workers that run in the background

An **agent** is a saved worker with a job, its own tools and a to-do list. It can run on a schedule or when asked, and it starts fresh every run.

### Create an agent

1. Click **Agents** in the sidebar, then **New agent**. Not sure where to start? Pick one of the examples at the top and change it.
2. Give it a **name**.
3. Write **what it should do**: its job, in plain words.
4. Choose **when it works**: only when asked, or on a schedule.
5. Click **Create agent**. The button stays at the bottom of the screen.

Those three answers are enough. Two optional boxes sit below them:

- **Tools and access:** let the agent use your Memory, Plan Book or Feed, and tick the tool servers (MCP) it may use. A ticked server shows a box: leave it empty to allow all of its tools, or type only the names it needs; **Show tools** lists them. While the box is closed, it says in one line what you chose.
- **Model and limits:** which AI model it uses, and how many steps and runs it may make.

If something is missing, the page tells you next to the field, and nothing you typed is lost. After creating, you land on the agent's page, where **Add its first task** and **Run it now** are waiting.

### Give it work

Open the agent and click **Tasks & settings**.

- **Add a task:** type a title, add details if needed, and click **Add task**. Tick **Start the agent now** if it should not wait for its schedule.
- **Start a run yourself:** click **Run now**, or type an instruction in the message box.
- **See what it did:** its page shows every run like a chat. Finished tasks show their result in the task list.

### Use an agent from a chat

1. On the home screen, click the **Agents** pill under the message box and tick one or more agents. In an existing chat, use the **Agents** button at the top.
2. Ask in normal words, for example: *"Ask Inbox helper to reply to Sam about the invoice."*
3. The AI hands the task to the agent. When the agent finishes, the result appears in the same chat.

Disconnecting an agent from a chat does not stop the agent.

### Give one agent its own tools

A dedicated server is used by one agent only. Your chats and other agents never see it. It works alongside the shared servers you ticked.

- **While creating the agent:** open **Tools and access**, then **Add a tool server only this agent uses**.
- **Later:** in **Tasks & settings**, find **This agent's own MCP servers**. Add as many as you need, test them, or remove them.

### Pause, limits and memory

- **Pause:** untick **Active** in the agent's settings, or click **Pause** on the Agents page.
- **Limits:** set the steps per run and the automatic runs per day under **Limits**. If the schedule asks for more runs than the daily limit allows (every 15 minutes is 96 a day), a note under the schedule offers to raise it. An agent that used up its runs for the day shows **Daily limit reached** and starts again at midnight UTC; **Run now** still works.
- **At a glance:** next to **Agents** in the sidebar, a number with a blinking dot is how many agents are working right now, and an amber number is how many need a look (the last run failed, or the daily limit is reached). They update by themselves. The Agents page shows the totals.
- **When something goes wrong:** if a run fails, or a tool server cannot be reached (the run then works without its tools), a note stays at the top of the agent's page with a **Run again** button, and the run itself is marked. Knitnox also writes one post in your Feed, marked **Problem**, and one more, **Working again**, when a run succeeds. It does not post after every failed run, and these posts do not count toward the agent's posts per day.
- **After the server was off:** a scheduled agent that missed its time runs once, shortly after the server starts, then continues on its schedule from there. Missed runs are not made up.
- **Adjustments:** short extra guidance on top of the instructions. A connected chat's AI may edit this part, never the instructions or tools. Every change is listed.
- **Its own notes:** notes the agent writes for its later runs. You can clear them.

### Let an agent use your Memory, Plan Book and Feed

An agent gets none of these unless you allow it. You choose them under **Built into Knitnox**: in **Tools and access** while creating the agent, or later in its **Setup** tab under **Which tools may it use?** (then **Save changes**).

- **Memory:** tick it to let the agent read what Knitnox remembers about you. It can never change it.
- **Plan Book:** choose whether the agent may read your tasks, or also add, change and delete them.
- **Feed:** tick it to let the agent post to your feed, and set how many posts a day it may add.

### The agent store

The **Agent store** tab has ready-made agents shared by people on your server.

- **Install one:** open it, read its guide and the tools it needs, then click **Add to my agents**. Your copy starts paused. Pick its model and your own tools, tick **Active**, and save.
- **Share yours:** open your agent, go to **Share in the agent store**, write a description and a short guide, and click **Publish**.

Sharing copies only the name, instructions, schedule and limits. Your tasks, memory, runs and MCP servers are never shared.

## Feed: short posts in one place

The **Feed** is one page of short posts, newest first. It is where you see at a glance what happened and what was found, without opening every chat and agent. Open **Feed** in the sidebar. The number next to it is how many posts you have not looked at yet.

The page shows only posts. Everything else is one click away in the row at the top.

| At the top | What it does |
|---|---|
| **Latest / Mine / Shared** | Which posts you see (see Views below) |
| **Everyone ▾** | Show only posts written by people, by the AI, by agents, or by one agent |
| **Find a post…** | Search the text of posts and reports |
| **Settings** (the sliders button) | Who may post, the models for pictures, voice and video, how the Feed works, and the bin |
| **New post** | Opens the box to write a post |

### Write and read posts

- **Post:** click **New post**, type, and click **Post**. A post is at most 500 characters.
- **Add a longer report:** open **Add a longer report** and write the details. They open behind **Read the full report**.
- **Let AI add:** when a picture, voice or video model is set up, you can ask for **A picture**, **Read it aloud** or **A video** with your post. The post appears at once; the picture, recording or video shows up on it by itself when it is ready.
- **Who can see it:** choose **Only me** or **Everyone on this server**. You can change this later from the post's **⋯** menu.
- **Older posts:** the page shows the newest 20. Scroll down and 10 more load each time you reach the end.
- **New posts:** when a post arrives while you are reading, a button such as **3 new posts** appears at the top. Click it to see them. Nothing moves until you do.

### Views

| View | What it shows |
|---|---|
| **Latest** | Your own posts and what others shared |
| **Mine** | Posts on your own feed, private and shared |
| **Shared** | Posts that people on this server shared with everyone |
| **Bin** | Posts you deleted, kept for 30 days. Open it from the settings button |

### Change, share and delete

Open the **⋯** menu on one of your posts.

- **Change the text:** edit the post and its report in place.
- **Share with everyone / Make it private again:** a shared post shows **Seen by N people**, the number of other people who were shown it.
- **Delete:** the post goes to the bin. Click **Undo** right away, or restore it later from the **Bin**.

There are no likes, comments or follows.

### Let the AI and agents post

Open the settings button, then **Who may post**. The AI cannot post until you click **Let the AI post**. After that, in your own chats:

- "Post a short summary of this to my feed."
- "Post this with a diagram of the steps."
- "What is on my feed about the invoice?"

To let an agent post, open the agent, go to **Setup** → **Which tools may it use?**, tick **Feed**, set its posts per day and save. When the limit is reached, the agent keeps working but posts nothing more that day.

What the AI and agents post is always private. Only you can share a post. They can read your own posts but never what other people shared. The feed is never used in group chats.

### What a post can carry

| Extra | Needs a model? | What it is |
|---|---|---|
| Report | No | A longer text behind **Read the full report** |
| Chart | No | Bars or a line, drawn from a few numbers the AI gives |
| Diagram | No | A picture the AI draws itself, for steps, timelines and how things connect |
| Picture | A picture model | An image made from a description |
| Voice | A voice model | The text read aloud |
| Video | A video model | A short film made from a description. It takes minutes |

When the AI or an agent asks for a picture and its own model can see images, it looks at the finished picture once. If the picture does not show what was asked for, it is drawn a second time and the post says so.

### Pictures, voice and video: choosing the models

Open the settings button, then **Pictures, voice and video**.

- **Pick a model for each kind**, or choose **Off**. "The server's model" is the one your admin shares with everyone.
- **Add my own model:** if you have your own account with an AI provider, type the model name, the provider's address and your key. The model is used for your feed right away. It is also listed under **Settings → Your API keys**, so you never type a key twice.
- **Limits:** with the server's models you can make a set number of pictures, recordings and videos per day (your admin chooses the number). Your own models are not limited.

The same small form is on the **Memory → Settings** page, to add your own embedding model for smart search.

## Plan Book: your tasks and projects

The **Plan Book** is your own to-do list. Open **Plan Book** in the sidebar. The number next to it is how many tasks are due today or late.

The page has three tabs: **Tasks** (your list), **Projects** and **Settings**.

### Add and finish tasks

Above your tasks you see today's date, one sentence about what is due, and the next seven days. The small number on a day is how many tasks are due then.

| I want to… | Do this |
|---|---|
| Add a task | Type it in the **Add a task** box and press **Enter** |
| Give it a date, a project or a note | Click in the box: the choices open under it. Pick **Today**, **Tomorrow**, **Next week** (the coming Monday) or **Pick a date…** before adding |
| Finish a task | Click the circle in front of it |
| Take back a wrong click | Click **Undo** in the line that appears after you add, finish, move or delete a task |
| Move a task to another day | Click its date (or **Add a date**) and choose **Today**, **Tomorrow**, **Next week** or **Another date…** |
| Catch up on late tasks | Next to **Late**, click **Move all to today** |
| See one day only | Click that day in the row of seven days. Click it again, or **Show everything**, to go back |
| Change a task, or see who changed it | Click its text, or **⋯** then **Change, or see its history** |
| Show that I am working on it | **⋯**, then **I am working on it** |
| Delete a task | **⋯**, then **Delete**. It waits in the bin for 30 days |
| Bring a deleted task back | Click the bin button next to **Find a task…**, then **Restore** |
| See what is urgent | **Today** shows tasks due today or late |
| Find a task | Type in **Find a task…** |

### Projects

A **project** groups tasks that belong together, like "Move house".

- **Create one:** open the **Projects** tab, click **New project**, give it a name and, if you like, a line about its goal.
- **Look at one:** click it on the **Projects** tab, or pick it in the **All tasks** menu above your list. You see its tasks and how many are done.
- **Rename or delete:** open the project, then **Rename or delete**. Deleting a project moves its tasks to the bin.

### Let the AI and agents help

The AI cannot see your Plan Book until you click **Let the AI use it** (on the banner, or on the **Settings** tab, where you can also turn it off again). After that, in your own chats:

- "Add a task to call Sam on Friday."
- "What is on my plan today?"
- "Mark the passport task as done."

The Plan Book is never used in group chats. To let an agent use it, open the agent, go to **Setup** → **Which tools may it use?** → **Plan Book**, and choose **Read them** or **Read, add, change and delete them**. Tasks added by the AI or an agent are labelled, and anything they delete goes to the bin.

## Memory: what the AI remembers about you

Without memory, every chat starts from zero. With it, the AI saves short facts you share and looks them up later, so you do not repeat yourself. Open **Memory** in the sidebar.

### Turn it on

Memory is **off** until you click **Turn on memory**. You can turn it off again under **Memory → Settings**. Turning it off deletes nothing.

| Where | What happens |
|---|---|
| Your own chats | The AI can save facts and look them up |
| Group chats | Memory is never used |
| Agents | Only an agent you allowed, and it can only read |

### Use it in a chat

- **Save something:** just tell the AI, or say "remember that I am vegetarian". It saves a short note and says so.
- **Use it later:** ask as usual. The AI searches its notes before it answers.
- **Fix it:** say "that is wrong, I moved to Lisbon" and it corrects the note, or change it yourself on the Memory page.

### Look after your memories

| I want to… | Do this |
|---|---|
| See everything | **Memory → Memories**. Newest first |
| Find one | Type in **Search your memory** |
| Add one myself | **Add a memory yourself** |
| Change one | Click the pencil on its card |
| Make sure it is never forgotten | Click the pin. Pinned memories are shown to the AI in every chat. Keep them few |
| Delete one | Click the bin |
| Delete everything, or download a copy | **Memory → Settings → Your data** |

### Things and connections

A **thing** is a person, place or project your memories name. A **connection** says how two things belong together, for example "Sam works at Acme Bakery".

- Things appear by themselves when a memory names them. When you add a memory, fill in **Who or what is this about?**
- To connect two things, open **Connect things to each other** in the form and fill the three boxes: a thing, how it relates, another thing.
- **Memory → Things & connections** lists every thing. Open one to see all that is known about it.
- The **map** shows things as circles. Click a circle to bring in everything it is linked to.
- Is one thing listed under two names? Open it, choose **Rename or merge**, and give it the other's exact name.

### Smart search

Memory always finds notes that share words with your question. With an **embedding model** it also finds notes that mean the same in other words: "what do I eat?" finds "I am vegetarian".

- Your admin can set one for everyone. **Memory → Settings** shows whether it is on.
- To use your own, add a provider under **Settings → Your API keys**, then pick it under **Memory → Settings** and type the model name, for example `text-embedding-3-small`. Click **Test it** first.

## Notifications and installing as an app

- **Notifications:** **Settings → Notifications → Enable on this device**. You are notified when an answer finishes while you are away, or when a tool needs your approval.
- **Install as an app:** use your browser's "Install" or "Add to Home Screen" option. On iPhone, add it to the Home Screen first, then enable notifications.

Both need the site to be served over `https://`.

## Your account

Under **Settings** you can change your username and your password (the current password is asked for, and a new password is typed twice), see how much of your token allowance you have used, add your own API keys, and **Log out everywhere**. Changing your password signs you out on your other devices.

**Forgot your password or username?** Ask an administrator of your Knitnox: they can tell you your username and set a new password for you under **Admin → Users**. There is no e-mail reset.

Only shared models count toward your allowance. Your own keys are never limited.

**Time zone.** The AI is told your local date and time, so "tomorrow" and "this evening" mean what you expect. Knitnox reads the time zone from your device. To fix it yourself, type it under **Settings → Account** (for example `Europe/Lisbon`) and untick **Set it from my device**. Group chats go by the server's clock, because people may be in different time zones.

## For admins

The first account created is the admin. Admins see **Admin panel** in the sidebar. After the first account, nobody else can sign up until you allow it: create accounts under **Users**, or tick **Allow anyone to sign up** under **Settings**.

| Tab | What you do there |
|---|---|
| **Providers** | Add shared AI providers and choose their models |
| **Users** | Create accounts, set token limits, rename, set a new password, disable or delete users |
| **Usage** | See tokens and cost per user and model |
| **Audit** | See who changed what in the admin panel |
| **Settings** | Sign-up, queue size, step and time limits, backups, housekeeping, the system prompt and more |

**Backups.** Knitnox makes a backup every day and keeps the newest seven. **Admin panel → Settings → Backups** shows when the last one ran and warns if it failed; **Back up now** makes one at once. Ask whoever runs the server to keep the backup folder on another disk.

**Someone forgot their password or username.** Open **Users**, click **⋯** next to the person, and type a new password or a new username. If you forgot your own, it is reset on the computer where Knitnox runs (`knitnox list-users`, `knitnox reset-password NAME NEW-PASSWORD`).

**Housekeeping.** Old usage and audit lines and old agent runs are removed after the limits under **Settings → Housekeeping**. Chats, memories, tasks and posts are never removed this way.

**Memory for everyone.** Under **Admin panel → Settings → Long-term memory**, pick a shared provider and type the name of its embedding model to give every user smart search. Click **Test it** before saving. Without it, memory still works with word search.

## When something goes wrong

| What you see | What to do |
|---|---|
| "No model selected" or "model is disabled" | Pick another model under **Share → Chat settings** |
| "Token quota exceeded" | Wait for the next period, ask an admin, or use your own API key |
| "Waiting for a free slot…" | Other chats are being answered first. It starts by itself |
| "Queue is full" | Wait for the AI to catch up, or remove a queued message |
| A tool card is waiting | Click **Allow once** or **Deny**. Unanswered cards are denied after a while |
| An agent does nothing | Check that it is **Active**, has a model, and has tools ticked |
| An answer stopped half way | Click **Retry** under it |
| "The provider stopped answering" or "the answer was cut off" | The AI provider went silent or hung up. What was written is kept. Click **Retry** |
| "Stopped after … minutes" | The answer took longer than the server allows. Try a shorter request, or ask an admin to raise the limit |
| "Sign-up is closed" | Ask an admin for an account |
| "Too many failed attempts" | Wait 15 minutes, or ask an admin to set a new password |
| Pages take very long to open | If you have many Knitnox tabs open, close some |

## Report a problem or an idea

Open **Help → Issues** and click **Report an issue**. Say what you did, what you expected and what happened. Everyone on your server can read issues, so never paste passwords or keys.
