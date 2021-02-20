This is a fork of [tdewolff/parse](https://github.com/tdewolff/parse) (MIT License).

I changed the following:
* Made the lexer streaming (main motivation for this fork)
* Makes errors less specific (error cases are the same, but the error messages aren't great), mostly because my package doesn't need them
* Removed the JavaScript parser as my package doesn't need it
