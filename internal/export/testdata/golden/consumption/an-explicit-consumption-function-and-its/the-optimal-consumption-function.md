[↑ An explicit consumption function and its properties](index.md)

# The optimal consumption function

As in <span class="citation" data-cites="Holm2018consumption">(Holm
2018)</span>, the value function for this problem solves the following
Hamilton-Jacobi-Bellman equation:
``` math
\begin{align}
\rho V(a) = \max_{c\geq 0} \left\{u(c) + V'(a)\cdot(ra+y-c)\right\}
\end{align}
```
subject to $`a\geq \underline{a}`$. Taking the first order conditions
with respect to $`c,a`$ and using the envelope condition one obtains
after re-arranging:
``` math
\begin{align}
c(a) = ra+y + \frac{\rho-r}{\gamma c'(a)}
\label{eq:ODE}
\end{align}
```
which is a special case of equation (7) in <span class="citation"
data-cites="Holm2018consumption">(Holm 2018)</span> who works with a
more general class of utility functions. Under this more general class,
he shows that this first order non-linear ODE is solvable and provides
an implicit expression for the consumption function. In contrast, the
specific utility function considered here is amenable to an explicit
closed form solution. This solution is described in the following
theorem:

> <span id="thm:cons_func">**Theorem**</span>
>
> **Theorem 1**. Assume that the Assumption
> <a href="preferences-and-technology.md#ass:parameters" data-reference-type="ref"
> data-reference="ass:parameters">1</a> holds and that
> $`\underline{a}>-y/r`$. It follows that the optimal consumption
> function that solves equation
> <a href="the-optimal-consumption-function.md" data-reference-type="eqref"
> data-reference="eq:ODE">[eq:ODE]</a> is given by:
> ``` math
> \begin{align}
> \label{eq:cstar_W0}
> c^\ast(a,y;r,\underline{a}) &= r a+y+\frac{\rho-r}{\gamma r}\left[1+W_0\left(f(a)\right)\right]\\
> f(a)& \vcentcolon=-e^{-\left[1+(a-\underline{a})\frac{\gamma r^2}{\rho-r}\right]}
> \end{align}
> ```
