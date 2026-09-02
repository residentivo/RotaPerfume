# Understand the Theory behind Clean Architecture

From a global perspective, Clean Architecture is a software design approach focused on structuring systems in a way that promotes clarity, maintainability, and flexibility.

It emphasizes separation of concerns and independence of implementation details. Its goals include creating systems that are easy to modify over time.

To be more precise, it prioritizes high-level policies and business rules, keeping them independent of low-level details like frameworks and databases, enabling testability, and long-term sustainability of the software.

A great starting point for Clean Architecture is Robert C. Martin (Uncle Bob) [blog post](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).

## History behind Clean Architecture

The concept of *Clean Architecture* was first formulated by Robert C. Martin in his blog post from 2012. It was also popularised when he published his book "[Clean Architecture: A Craftsman's Guide to Software Structure and Design](https://www.amazon.com/Clean-Architecture-Craftsmans-Software-Structure/dp/0134494164)" in 2017.

While Robert C. Martin was the one to define Clean Architecture, he didn't invent all the concepts it's built upon.

### Hexagonal Architecture

One of the first references from the original blog post is *Hexagonal Architecture* (also called *Ports & Adapters*). It was originally a [blog post](https://alistair.cockburn.us/hexagonal-architecture/), written by Alistair Cockburn, in 2005.

Essentially, it aims to decouple core business logic of an application from its external dependencies, such as databases, user interfaces, and external services.

The solution suggested by Alistair Cockburn is to define a clear boundary between the core logic and the external interactions, through a set of "ports" and "adapters".

![](https://cdn.hashnode.com/res/hashnode/image/upload/v1710267536768/3fe719fa-df9c-41c5-a245-8e68666dd16a.png align="center")

Ports represent interfaces or contracts that define how the core logic communicates with the external world. On the other hand, adapters are implementations of these ports.

### Onion Architecture

A few years later, in 2008, Jeffrey Palermo published about [Onion Architecture](https://jeffreypalermo.com/2008/07/the-onion-architecture-part-1/) on his blog. It also aims to decouple business logic from external dependencies but in a different way.

It does so by organizing the application into concentric layers, with the innermost layer representing the domain model or core business logic. This is how Jeffrey manages to isolate the core from external concerns like databases, frameworks, or UI.

To be precise, he defines 4 layers:

* **Core (Domain Model)**: This innermost layer contains the domain model and business logic of the application. It encapsulates the essential behavior and rules of the system.
    
* **Infrastructure (Domain Services)**: Surrounding the core, it provides implementations for interfaces defined in the core layer. It deals with external concerns like database access, framework integrations, and other infrastructure-related operations.
    
* **Application Services**: This layer orchestrates the interaction between the core domain and the infrastructure. It contains use cases or application-specific operations that coordinate the flow of data and actions within the system.
    
* **Presentation/UI**: It's the outermost layer, which handles user interface concerns. It includes components responsible for user interaction like controllers, views, or API endpoints.
    

![](https://cdn.hashnode.com/res/hashnode/image/upload/v1710270264346/c3c9498d-c30e-48b2-977f-224ea8213d41.png align="center")

In practice, Onion Architecture relies heavily on Dependency Injection. That's how the core can be independent from external implementations, and rely on abstractions defined by interfaces instead. This allows for loose coupling and easier substitution of components.

### Others

Robert C. Martin talks about two main books, [Lean Architecture: for Agile Software Development](https://www.amazon.com/Lean-Architecture-Agile-Software-Development/dp/0470684208/) and [Object-Oriented Software Engineering: A Use Case Driven Approach](https://www.amazon.com/Object-Oriented-Software-Engineering-Approach/dp/0201544350).

Both of those books influenced Clean Architecture but for different reasons.

**Lean Architecture**

In the first one, you'll read about [Data, context and interaction](https://en.wikipedia.org/wiki/Data,_context_and_interaction). The main point is to separate the domain model (data) from use cases (context) and roles that objects play (interaction).

**Object-Oriented Software Engineering**

Then, in the second book, we focus on [use cases](https://en.wikipedia.org/wiki/Use_case). You can see them as a list of actions defining the interactions between an actor and a system, to achieve a goal.

**Screaming Architecture**

Another approach can be found as part of Clean Architecture: [Screaming Architecture](https://blog.cleancoder.com/uncle-bob/2011/09/30/Screaming-Architecture.html). It's a concept formulated by Robert C. Martin, who says:

> If the plans you are looking at are for a single family residence, then you’ll likely see a front entrance, a foyer leading to a living room and perhaps a dining room. \[...\] As you looked at those plans, there’d be no question that you were looking at a *house*. The architecture would *scream*: **house**.

Then, he asks:

> So what does the architecture of your application scream? When you look at the top level directory structure, and the source files in the highest level package; do they scream: **Health Care System**, or **Accounting System**, or **Inventory Management System**? Or do they scream: **Rails**, or **Spring/Hibernate**, or **ASP**?

He's not only asking rhetorical questions but has a great conclusion for us:

> Just as the plans for a house or a library scream about the use cases of those buildings, so should the architecture of a software application scream about the use cases of the application.

**Humble Object**

If you read "Clean Architecture: A Craftsman's Guide to Software Structure and Design", you will learn about a new pattern: [Humble Object](http://xunitpatterns.com/Humble%20Object.html). It was originally popularised by Gerard Meszaros in [xUnit Test Patterns: Refactoring Test Code](https://www.amazon.com/xUnit-Test-Patterns-Refactoring-Code/dp/0131495054).

The idea is to separate the behavior that's hard to test from the behaviour that's easy to test. It's achieved by splitting a functionality into two parts:

* A humble part kept as simple as possible, that contains the hard-to-test code (usually dealing with problematic dependencies).
    
* A part that contains the logic or behavior that can be easily tested.
    

**Component Design Principles**

From a general perspective, large systems are built upon smaller components that are working together.

In his book on Clean Architecture, Uncle Bob gives us a set of principles that define how component should be composed together. While this is a broader subject, I definitely recommend you [read about it](https://blog.scalablebackend.com/understand-component-design-principles).

## Clean Architecture

There are a few concepts and rules that make Clean Architecture what it is today, as well as its specific layer organization. To get started, you can have a look at the following diagram:

![](https://cdn.hashnode.com/res/hashnode/image/upload/v1710343480050/57af3ed5-0205-4cab-b206-686fc9431c5d.png align="center")

I highly recommend keeping this diagram saved somewhere (or the original one from Robert C. Martin [blog post](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)). Thankfully, you don't have to understand everything about this diagram yet.

Right now, you will discover what those layers are, but will understand them more in-depth through practice. With time, you will come back to look at this illustration and find how much it makes sense.

As you can see, there are four layers, from the innermost to the outermost:

* **Entities**: They represent the essential business objects or concepts that are relevant to the application. In practice, an entity encapsulates the most general and high-level business rules and logic. They are completely independent of external concerns and are essentially the heart of the application's business logic.
    
* **Use Cases (or Interactors)**: Use cases represent the application's behavior and define the actions that can be performed within the system. They encapsulate the workflow or the specific steps required to achieve a particular goal or perform a task. To be more precise, use cases orchestrate the flow of data and operations between the entities and the outer layers. Each use case typically corresponds to a specific user action or system operation.
    
* **Interface Adapters**: They act as the intermediary between the inner layers (Entities and Use Cases) and the outer layers (Frameworks and Drivers). They are responsible for translating data from the format most convenient for the use cases and entities into the format most convenient for the external frameworks and tools. This layer usually includes controllers in a traditional MVC architecture, as well as data mappers and gateways for external services.
    
* **Frameworks and Drivers**: The outmost layer consists of the external frameworks, tools, and devices such as the database, web framework, UI, etc. It deals with the delivery mechanisms and frameworks specific to the platform. It includes components such as web frameworks (Front-End or Back-End), databases, and other external systems and devices.
    

> It's a common practice to call the group with Entities & Use Case "Business" or "Domain", while the group with Interface Adapters & Frameworks is called "Infrastructure".

### The dependency rule

Probably the most important concept from Clean Architecture. In his blog post, Uncle Bob says:

> This rule says that *source code dependencies* can only point *inwards*. Nothing in an inner circle can know anything at all about something in an outer circle.
> 
> In particular, the name of something declared in an outer circle must not be mentioned by the code in the inner circle. That includes functions, classes. variables, or any other named software entity.
> 
> By the same token, data formats used in an outer circle should not be used by an inner circle, especially if those formats are generate by a framework in an outer circle. We don’t want anything in an outer circle to impact the inner circles.

Now, you might ask: how is this possible? For example, when a use case needs to communicate through an adapter (defined inside or outside of its layer).

This issue is solved thanks to Dependency Injection. Usually, an interface is defined inside an inner layer, and implemented inside the outer layer. This structure is illustrated in the bottom right of the Clean Architecture diagram.

There, we can see the use case (or interactor) uses a presenter, which belongs to an outer layer. In order to conform to the Dependency Rule, the use case depends on an interface defined in its own layer. The actual presenter is implemented in the outer layer.

### Common misconception

**Data Access**

While it's tempting to define data access gateways inside the Framework & Drivers layer, that's not where they belong! Simply because you use a framework or driver to request your database doesn't make your gateway part of the outermost layer.

The important characteristic is how data flows: it's a gateway for data access. It's pretty obvious in the Clean Architecture diagram: it belongs to the Interface Adapters layer.

**Controller**

Often, if you try to adhere strictly to Clean Architecture, you will have a problem with the definition of your controller. In most applications, controllers use some sort of functionality from a framework, making it incompatible with the interface adapter layer.

The truth is: controllers can have a role that spans multiple layers. They are usually a part of both the Framework & Driver and Interface Adapters layer. In practice, you could make them independent of the Framework & Driver layer by:

* Splitting every controller in two (one in each layer)
    
* Converting the response format from one layer to the other.
    

Again, in practice, this is a terrible idea that has multiple downsides without bringing any real value.

**Presenter**

The presenter is not a common concept and can be quite confusing at first. It's defined by Robert C. Martin in cooperation with the view (in the context of UI) and is introduced at the same time as the Humble Object pattern.

That's because the view should be treated as the hard-to-test, humble object, while the presenter is the testable object. To be more precise, the presenter is responsible for formatting data that is passed to the view.

*That also means the view should be kept as simple as possible.*

The presenter is sometimes confused with the controller in Front-End applications. They have a completely different role: while the controller orchestrates the actions taken by an actor with the use cases, the presenter formats data received by the view.

**Crossing boundaries**

Uncle Bob is very clear when it comes to data crossing boundaries. Not only should you use Dependency Injection to respect the Dependency Rule, but you should also be careful about what type of data goes from one layer to another.

In his blog post, we can read:

> Typically the data that crosses the boundaries is simple data structures. You can use basic structs or simple Data Transfer objects if you like.

You don't want some data structure that comes from a framework or library to go around your different layers. Instead, you want to convert them to simple data structure.

In the same blog post, there is also:

> We don’t want to cheat and pass *Entities* or Database rows.

Be careful about entities, this is not an absolute rule. For example, the default practice is to return entities from your data access gateway (usually *repositories* by the way).

On the other hand, 99% of the time, you don't want your entity to leave the use case layer. An acceptable exception could be if you have specific entities related to authentication, which are heavily used at the controller level.

Otherwise, most of the time your entities leave the use case layer, it's an architecture smell.

**Only 4 layers?**

While Clean Architecture is presented with 4 amazing layers, it can be changed depending on your application needs. While it's not common, removing a layer can simplify your architecture, even though it could lead to decreased modularity and flexibility.

Adding a layer is also entirely possible. Sometimes we can find an added Domain layer, which sits between the Entities and Use Cases layers.

The Domain layer typically contains domain-specific logic and rules that are fundamental to the problem domain being of the application. This layer helps to keep the business logic isolated and cohesive, making it easier to understand and maintain.

### In practice

I highly recommend you have a look a two other articles that explain how to implement Clean Architecture. They are both a Tic-Tac-Toe game for:

* [Front-End with ReactJS](https://morintd.hashnode.dev/build-a-react-application-with-clean-architecture)
    
* [Back-End with ExpressJS](https://blog.scalablebackend.com/build-an-expressjs-application-with-clean-architecture)
    

## What Clean Architecture is not

### Domain-Driven-Design (DDD)

Often, we find concepts that originate from DDD discussed in Clean Architecture spaces. For examples:

* Domain services
    
* Domain events
    
* Domain-specific value objects
    

Are all rooted in Domain-Driven-Design. While they can be a great addition to Clean Architecture, they're not strictly a part of it.

### CQS/CQRS

Often, instead of use cases, you will find commands and queries in applications made with Clean Architecture. They are principles that come from [CQS](https://en.wikipedia.org/wiki/Command%E2%80%93query_separation) & [CQRS](https://en.wikipedia.org/wiki/Command_Query_Responsibility_Segregation), which have a great synergy with Clean Architecture.

Similarly to concepts from DDD, they are homework not directly a part of Clean Architecture.

## Difference with Onion Architecture

Even though Onion Architecture looks similar to Clean Architecture, there are fundamental differences between both:

* **Layer Organization**: Onion Architecture organizes layers based on their roles (core, infrastructure, application, presentation), while Clean Architecture organizes layers based on the flow of data and dependencies (entities, use cases, interface adapters, frameworks/drivers).
    
* **Dependency Direction**: In Onion Architecture, dependencies are typically inverted, with the core depending on abstractions defined in higher-level layers. In Clean Architecture, dependencies flow inward toward higher-level policies and business rules.
    
* **Focus**: Onion Architecture focuses on separating concerns through layered architecture, while Clean Architecture emphasizes dependency management and separation of concerns based on data flow.
    

---

Are looking for more in-depth resources about ***Clean Architecture***? Let me know [here](https://www.scalablebackend.com/courses/everything-about-clean-architecture-on-the-backend).