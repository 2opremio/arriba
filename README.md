# Arriba! - A Slack bot for asynchronous stand-ups

Arriba! (Up! in Spanish) keeps track of your
[stand-up](https://en.wikipedia.org/wiki/Stand-up_meeting) messages in
[Slack](https://slack.com/). Here's an example:

![Stand-up example with Arriba](img/screenshot.png)


## Using arriba

### 1. Get a Slack bot token

Go to `https://<yourteam>.slack.com/services/new/bot`

![New bot](img/newbot.png)

Click on *Create bot integration*

![Bot token](img/bottoken.png)

The bot token is `xoxb-12803307735-NIlz3RAH1kBf0ir47oYhGejr` in the example above.


### 2. Run arriba

You have two options

1. Compile it yourself (you will need an installation of [Go](https://golang.org/)):

   ```bash
go get github.com/2opremio/arriba
arriba xoxb-12803307735-NIlz3RAH1kBf0ir47oYhGejr
    ```

2. Use Docker:

   ```bash
docker run 2opremio/arriba xoxb-12803307735-NIlz3RAH1kBf0ir47oYhGejr

   ```

### 3. Talk to him!

Arriba should now be connected to your Slack team. Just invite him to channels
and use it as in the example above.
